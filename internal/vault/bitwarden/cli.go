// Package bitwarden adapts the official bw CLI for Bitwarden and Vaultwarden.
package bitwarden

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lazypass/internal/debug"
	"lazypass/internal/vault"
)

const DefaultServer = "https://vault.bitwarden.com"

type Runner interface {
	Run(context.Context, []string, io.Reader, []string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, args []string, input io.Reader, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "bw", append(append([]string(nil), args...), "--nointeraction")...)
	// Never inherit another account's session or login credentials.
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		switch key {
		case "BW_SESSION", "BW_PASSWORD", "BW_CLIENTID", "BW_CLIENTSECRET", "LAZYPASS_BW_PASSWORD", "BITWARDENCLI_APPDATA_DIR":
			continue
		}
		cmd.Env = append(cmd.Env, item)
	}
	cmd.Env = append(cmd.Env, env...)
	cmd.Stdin = input
	return cmd.Output()
}

type Prompt func() (string, error)

type Provider struct {
	DataDir       string
	Server        string
	Prompt        Prompt
	Runner        Runner
	mu            sync.RWMutex
	session       string
	metadata      *snapshot
	cloneAttempts map[[32]byte]*cloneAttempt
	// Published snapshots are immutable; mutations invalidate metadata but retain
	// the last browser snapshot until reconciliation or explicit locking.
	browser atomic.Pointer[snapshot]
}

func New(dataDir, server string, prompt Prompt, runner Runner) *Provider {
	if server == "" {
		server = DefaultServer
	}
	if runner == nil {
		runner = ExecRunner{}
	}
	return &Provider{DataDir: dataDir, Server: strings.TrimRight(server, "/"), Prompt: prompt, Runner: runner}
}

func (p *Provider) Name() string           { return "bitwarden" }
func (p *Provider) UsesPassphrasePrompt()  {}
func (p *Provider) StoreDirectory() string { return p.DataDir }
func (p *Provider) Capabilities() vault.Capabilities {
	return vault.Capabilities{Sync: true, Delete: true}
}
func (p *Provider) Lock() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.session = ""
	p.cloneAttempts = nil
	p.metadata = nil
	p.browser.Store(nil)
}

func (p *Provider) run(ctx context.Context, args []string, input io.Reader, extra ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if checker, ok := p.Runner.(serverAccessChecker); ok {
		if err := checker.CheckServerAccess(ctx, p.Server); err != nil {
			debug.Failure("bitwarden server access", debug.Unavailable)
			return nil, err
		}
	}
	env := []string{"BITWARDENCLI_APPDATA_DIR=" + p.DataDir, "BW_NOINTERACTION=true"}
	if p.session != "" {
		env = append(env, "BW_SESSION="+p.session)
	}
	env = append(env, extra...)
	output, err := p.Runner.Run(ctx, args, input, env)
	if err != nil {
		cause := debug.Unavailable
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			cause = debug.Timeout
		}
		debug.Failure("bitwarden command", cause)
		classified := classifyFailure(ctx, output, err)
		if errors.Is(classified, vault.ErrUninitialized) {
			classified = p.signedOut()
		}
		if errors.Is(classified, vault.ErrLocked) || errors.Is(classified, vault.ErrUninitialized) {
			p.session = ""
			p.metadata = nil
			p.browser.Store(nil)
		}
		return nil, classified
	}
	return output, nil
}

type status struct {
	UserID    string `json:"userId"`
	Status    string `json:"status"`
	ServerURL string `json:"serverUrl"`
}

func (p *Provider) BrowserCacheIdentity(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.status(ctx)
	if err != nil {
		return "", err
	}
	if s.Status == "unauthenticated" || s.UserID == "" {
		return "", p.signedOut()
	}
	return p.Server + "\x00" + s.UserID, nil
}

func (p *Provider) status(ctx context.Context) (status, error) {
	var result status
	output, err := p.run(ctx, []string{"status"}, nil)
	if err != nil {
		return result, err
	}
	if json.Unmarshal(output, &result) != nil {
		return result, vault.ErrMalformed
	}
	server := strings.TrimRight(result.ServerURL, "/")
	if server == "" {
		server = DefaultServer
	}
	if server != p.Server {
		return result, vault.ErrConflict
	}
	return result, nil
}

func (p *Provider) Status(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.status(ctx)
	if err != nil {
		return err
	}
	if s.Status == "unauthenticated" {
		return vault.ErrUninitialized
	}
	if s.Status != "locked" && s.Status != "unlocked" {
		return vault.ErrMalformed
	}
	return nil
}

func (p *Provider) ensureUnlocked(ctx context.Context) error {
	if p.session != "" {
		return nil
	}
	s, err := p.status(ctx)
	if err != nil {
		return err
	}
	if s.Status == "unauthenticated" {
		return p.signedOut()
	}
	if s.Status != "locked" && s.Status != "unlocked" {
		return vault.ErrMalformed
	}
	if vault.IsNoninteractive(ctx) {
		return vault.ErrPromptRequired
	}
	password, err := vault.RequestPassphrase(ctx, p.Prompt)
	if err != nil {
		return err
	}
	output, err := p.run(ctx, []string{"unlock", "--passwordenv", "LAZYPASS_BW_PASSWORD", "--raw"}, nil, "LAZYPASS_BW_PASSWORD="+password)
	defer clear(output)
	if err != nil {
		return err
	}
	session := strings.TrimSpace(string(output))
	if session == "" || strings.ContainsAny(session, "\r\n\x00") {
		return vault.ErrLocked
	}
	p.session = session
	return nil
}

// UnlockEarly collects input before CLI startup/network latency, then uses the
// normal status validation and unlock path without persisting the passphrase.
func (p *Provider) UnlockEarly(ctx context.Context) error {
	if p.mu.TryLock() {
		unlocked := p.session != ""
		p.mu.Unlock()
		if unlocked {
			return nil
		}
	}
	password, err := vault.RequestPassphrase(ctx, p.Prompt)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ensureUnlocked(vault.WithPassphrasePrompt(ctx, func(context.Context) (string, error) {
		return password, nil
	}))
}

func (p *Provider) Initialize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := os.MkdirAll(p.DataDir, 0700); err != nil {
		return vault.ErrUnavailable
	}
	output, err := p.run(ctx, []string{"status"}, nil)
	if err != nil {
		return err
	}
	var s status
	if json.Unmarshal(output, &s) != nil {
		return vault.ErrMalformed
	}
	if s.Status != "unauthenticated" {
		return vault.ErrConflict
	}
	_, err = p.run(ctx, []string{"config", "server", p.Server}, nil)
	return err
}

func (p *Provider) Sync(ctx context.Context) (vault.SyncResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() { p.metadata = nil }()
	if err := p.ensureUnlocked(ctx); err != nil {
		return vault.SyncResult{}, err
	}
	_, err := p.run(ctx, []string{"sync"}, nil)
	return vault.SyncResult{Pulled: err == nil}, err
}

func decode[T any](output []byte, err error) (T, error) {
	var result T
	if err != nil {
		return result, err
	}
	if json.Unmarshal(bytes.TrimSpace(output), &result) != nil {
		return result, vault.ErrMalformed
	}
	return result, nil
}
