// Package pass adapts the pass command line program to the vault contract.
package pass

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lazypass/internal/debug"
	"lazypass/internal/vault"

	"golang.org/x/term"
)

const commandTimeout = 2 * time.Minute

type Runner interface {
	Run(context.Context, string, []string, io.Reader, []string) ([]byte, error)
}

type ExecRunner struct{ Terminal io.Reader }

func (r ExecRunner) Run(ctx context.Context, name string, args []string, stdin io.Reader, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin == nil {
		stdin = r.Terminal
		if stdin == nil {
			stdin = os.Stdin
		}
	}
	cmd.Stdin = stdin
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.Stderr, err
		}
	}
	return output, err
}

type Provider struct {
	StoreDir string
	Runner   Runner
}

func New(storeDir string, runner Runner) *Provider {
	if runner == nil {
		runner = ExecRunner{}
	}
	return &Provider{StoreDir: storeDir, Runner: runner}
}

func (p *Provider) Name() string { return "pass" }

// StoreDirectory returns the configured or resolved pass store location.
func (p *Provider) StoreDirectory() string { return p.storeDir() }

func (p *Provider) Capabilities() vault.Capabilities {
	return vault.Capabilities{Sync: true, Delete: true}
}

// Status reports whether pass can run and the selected store directory exists.
func (p *Provider) Status(ctx context.Context) error {
	if _, err := p.run(ctx, []string{"--version"}, nil); err != nil {
		return err
	}
	info, err := os.Stat(p.storeDir())
	if err != nil || !info.IsDir() {
		debug.Failure("pass status", debug.Unavailable)
		return vault.ErrUnavailable
	}
	if _, err := os.Stat(filepath.Join(p.storeDir(), ".gpg-id")); err != nil {
		debug.Failure("pass status", debug.Invalid)
		return vault.ErrUninitialized
	}
	return nil
}

func (p *Provider) List(ctx context.Context, path vault.Path) ([]vault.Node, error) {
	if err := path.Validate(); err != nil {
		return nil, err
	}
	root, err := p.safeDirectory(path)
	if err != nil {
		logPathFailure("pass list", err)
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		debug.Failure("pass list", debug.IO)
		return nil, vault.ErrUnavailable
	}
	nodes := make([]vault.Node, 0, len(entries))
	for _, item := range entries {
		if item.Type()&os.ModeSymlink != 0 {
			continue
		}
		name := item.Name()
		if item.IsDir() {
			if err := (vault.Path{name}).Validate(); err == nil {
				nodes = append(nodes, vault.Node{Path: appendPath(path, name), Name: name, Kind: vault.FolderNode})
			}
			continue
		}
		if !strings.HasSuffix(name, ".gpg") {
			continue
		}
		info, err := item.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		name = strings.TrimSuffix(name, ".gpg")
		if err := (vault.Path{name}).Validate(); err == nil {
			nodes = append(nodes, vault.Node{Path: appendPath(path, name), Name: name, Kind: vault.EntryNode})
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Kind != nodes[j].Kind {
			return nodes[i].Kind == vault.FolderNode
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

func (p *Provider) Read(ctx context.Context, path vault.Path) (vault.Entry, error) {
	if err := requireEntryPath(path); err != nil {
		return vault.Entry{}, err
	}
	if err := p.safeEntry(path, true); err != nil {
		logPathFailure("pass show", err)
		return vault.Entry{}, err
	}
	output, err := p.run(ctx, append([]string{"show", "--"}, path.String()), nil)
	if err != nil {
		return vault.Entry{}, err
	}
	entry, err := vault.DecodeEntry(path, string(output))
	if err != nil {
		debug.Failure("pass decode", debug.Invalid)
	}
	return entry, err
}

func (p *Provider) Write(ctx context.Context, entry vault.Entry) error {
	if err := requireEntryPath(entry.Path); err != nil {
		return err
	}
	if err := p.safeEntry(entry.Path, false); err != nil {
		logPathFailure("pass insert", err)
		return err
	}
	encoded, err := vault.EncodeEntry(entry)
	if err != nil {
		debug.Failure("pass encode", debug.Invalid)
		return err
	}
	_, err = p.run(ctx, append([]string{"insert", "--multiline", "--force", "--"}, entry.Path.String()), strings.NewReader(encoded))
	return err
}

func (p *Provider) Delete(ctx context.Context, path vault.Path) error {
	if err := requireEntryPath(path); err != nil {
		return err
	}
	if err := p.safeEntry(path, true); err != nil {
		logPathFailure("pass remove", err)
		return err
	}
	_, err := p.run(ctx, append([]string{"rm", "--force", "--"}, path.String()), nil)
	return err
}

func (p *Provider) Sync(ctx context.Context) (vault.SyncResult, error) {
	if _, err := p.run(ctx, []string{"git", "pull", "--rebase"}, nil); err != nil {
		return vault.SyncResult{}, err
	}
	if _, err := p.run(ctx, []string{"git", "push"}, nil); err != nil {
		return vault.SyncResult{Pulled: true}, err
	}
	return vault.SyncResult{Pulled: true, Pushed: true}, nil
}

func (p *Provider) run(parent context.Context, args []string, stdin io.Reader) ([]byte, error) {
	if err := parent.Err(); err != nil {
		debug.Failure(passOperation(args), contextCause(err))
		return nil, vault.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()
	output, err := p.Runner.Run(ctx, "pass", args, stdin, []string{"PASSWORD_STORE_DIR=" + p.storeDir()})
	if err == nil {
		return output, nil
	}
	if ctx.Err() != nil {
		debug.Failure(passOperation(args), contextCause(ctx.Err()))
		return nil, vault.ErrUnavailable
	}
	message := strings.ToLower(err.Error() + " " + string(output))
	exitCode := processExitCode(err)
	var classified error
	cause := debug.Unknown
	switch {
	case strings.Contains(message, "not initialized"):
		classified = vault.ErrUninitialized
		cause = debug.Invalid
	case strings.Contains(message, "executable"), strings.Contains(message, "command not found"):
		classified = vault.ErrUnavailable
		cause = debug.Unavailable
	case strings.Contains(message, "not in the password store"):
		classified = vault.ErrNotFound
		cause = debug.NotFound
	case strings.Contains(message, "conflict"):
		classified = vault.ErrConflict
		cause = debug.Conflict
	case strings.Contains(message, "gpg"), strings.Contains(message, "secret key"), strings.Contains(message, "pinentry"), strings.Contains(message, "inappropriate ioctl"), strings.Contains(message, "decryption"):
		reason := gpgReason(message)
		classified = &GPGFailure{Reason: reason, ExitCode: exitCode}
		cause = reason.detail().cause
	case strings.Contains(message, "not found"):
		classified = vault.ErrNotFound
		cause = debug.NotFound
	default:
		classified = fmt.Errorf("pass command failed")
	}
	if debug.Enabled() {
		debug.ProcessFailure(passOperation(args), cause, debug.ProcessState{
			ExitCode: exitCode,
			StdinTTY: term.IsTerminal(int(os.Stdin.Fd())),
			GPGTTY:   os.Getenv("GPG_TTY") != "",
			SSH:      os.Getenv("SSH_TTY") != "",
			Display:  os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "",
		})
	}
	return nil, classified
}

func processExitCode(err error) int {
	var exit interface{ ExitCode() int }
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}

func contextCause(err error) debug.Cause {
	if errors.Is(err, context.DeadlineExceeded) {
		return debug.Timeout
	}
	return debug.Canceled
}

func passOperation(args []string) debug.Operation {
	if len(args) == 0 {
		return "pass"
	}
	switch args[0] {
	case "show":
		return "pass show"
	case "insert":
		return "pass insert"
	case "rm":
		return "pass remove"
	case "git":
		return "pass git"
	case "--version":
		return "pass version"
	default:
		return "pass"
	}
}

func logPathFailure(operation debug.Operation, err error) {
	cause := debug.Unavailable
	switch {
	case errors.Is(err, vault.ErrInvalidPath):
		cause = debug.Invalid
	case errors.Is(err, vault.ErrNotFound):
		cause = debug.NotFound
	}
	debug.Failure(operation, cause)
}

func (p *Provider) storeDir() string {
	if p.StoreDir != "" {
		return expandHome(p.StoreDir)
	}
	if dir := os.Getenv("PASSWORD_STORE_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".password-store"
	}
	return filepath.Join(home, ".password-store")
}

func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}

func (p *Provider) safeDirectory(path vault.Path) (string, error) {
	root, err := p.resolvedStoreDir()
	if err != nil {
		return "", err
	}
	for _, part := range path {
		root = filepath.Join(root, part)
		info, err := os.Lstat(root)
		if errors.Is(err, os.ErrNotExist) {
			return "", vault.ErrNotFound
		}
		if err != nil {
			return "", vault.ErrUnavailable
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", vault.ErrInvalidPath
		}
		if !info.IsDir() {
			return "", vault.ErrNotFound
		}
	}
	return root, nil
}

func (p *Provider) safeEntry(path vault.Path, mustExist bool) error {
	if len(path) == 0 {
		return vault.ErrInvalidPath
	}
	parent, err := p.safeDirectory(path[:len(path)-1])
	if err != nil {
		if !mustExist && errors.Is(err, vault.ErrNotFound) {
			return nil
		}
		return err
	}
	entry := filepath.Join(parent, path[len(path)-1]+".gpg")
	info, err := os.Lstat(entry)
	if errors.Is(err, os.ErrNotExist) {
		if !mustExist {
			return nil
		}
		return vault.ErrNotFound
	}
	if err != nil {
		return vault.ErrUnavailable
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return vault.ErrInvalidPath
	}
	if !info.Mode().IsRegular() {
		return vault.ErrNotFound
	}
	return nil
}

func (p *Provider) resolvedStoreDir() (string, error) {
	root, err := filepath.EvalSymlinks(p.storeDir())
	if err != nil {
		return "", vault.ErrUnavailable
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", vault.ErrUnavailable
	}
	return root, nil
}

func requireEntryPath(path vault.Path) error {
	if len(path) == 0 {
		return vault.ErrInvalidPath
	}
	return path.Validate()
}

func appendPath(path vault.Path, name string) vault.Path {
	result := append(vault.Path(nil), path...)
	return append(result, name)
}

var _ vault.Provider = (*Provider)(nil)
