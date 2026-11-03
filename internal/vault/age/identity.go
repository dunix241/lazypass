// Package age implements per-entry age encryption with a session-unlocked identity.
package age

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	agecrypto "filippo.io/age"
	"lazypass/internal/vault"
)

const manifestName = "vault.json"
const maxFileBytes = 2 << 20

type manifest struct {
	Version    int      `json:"version"`
	Recipients []string `json:"recipients"`
}
type Prompt func() (string, error)
type Provider struct {
	StoreDir     string
	IdentityFile string
	Prompt       Prompt
	mu           sync.Mutex
	identities   []agecrypto.Identity
}

func New(storeDir, identityFile string, prompt Prompt) *Provider {
	return &Provider{StoreDir: expand(storeDir), IdentityFile: expand(identityFile), Prompt: prompt}
}
func expand(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
func (p *Provider) Name() string                           { return "age" }
func (p *Provider) SupportedEntryTypes() []vault.EntryType { return vault.AllEntryTypes() }
func (p *Provider) UsesPassphrasePrompt()                  {}
func (p *Provider) StoreDirectory() string                 { return p.StoreDir }
func (p *Provider) Capabilities() vault.Capabilities       { return vault.Capabilities{Delete: true} }
func (p *Provider) Sync(context.Context) (vault.SyncResult, error) {
	return vault.SyncResult{}, vault.ErrUnsupported
}
func (p *Provider) Lock() { p.mu.Lock(); defer p.mu.Unlock(); p.identities = nil }

func (p *Provider) Status(ctx context.Context) error {
	r, err := p.open(ctx)
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = readManifest(r)
	return err
}

func readManifest(root *os.Root) (manifest, error) {
	var m manifest
	data, err := readRegular(root, manifestName)
	if errors.Is(err, vault.ErrNotFound) {
		return m, vault.ErrUninitialized
	}
	if err != nil {
		return m, err
	}
	if json.Unmarshal(data, &m) != nil || m.Version != 1 || len(m.Recipients) == 0 {
		return m, vault.ErrMalformed
	}
	for _, value := range m.Recipients {
		if _, err := agecrypto.ParseX25519Recipient(value); err != nil {
			return m, vault.ErrMalformed
		}
	}
	return m, nil
}

// Initialize writes the encrypted identity first; no plaintext private key is persisted.
func Initialize(storeDir, identityFile, passphrase string) error {
	storeDir, identityFile = expand(storeDir), expand(identityFile)
	if passphrase == "" || storeDir == "" || identityFile == "" {
		return vault.ErrInvalidPath
	}
	absStore, err := filepath.Abs(storeDir)
	if err != nil {
		return vault.ErrUnavailable
	}
	absIdentity, err := filepath.Abs(identityFile)
	if err != nil {
		return vault.ErrUnavailable
	}
	rel, err := filepath.Rel(absStore, absIdentity)
	if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return vault.ErrInvalidPath
	}
	if _, err := os.Lstat(filepath.Join(storeDir, manifestName)); !errors.Is(err, os.ErrNotExist) {
		return vault.ErrConflict
	}
	key, err := agecrypto.GenerateX25519Identity()
	if err != nil {
		return vault.ErrUnavailable
	}
	recipient, err := agecrypto.NewScryptRecipient(passphrase)
	if err != nil {
		return vault.ErrLocked
	}
	var encrypted bytes.Buffer
	w, err := agecrypto.Encrypt(&encrypted, recipient)
	if err != nil {
		return vault.ErrUnavailable
	}
	if _, err = w.Write([]byte(key.String() + "\n")); err != nil {
		return vault.ErrUnavailable
	}
	if w.Close() != nil {
		return vault.ErrUnavailable
	}
	if err := os.MkdirAll(filepath.Dir(identityFile), 0700); err != nil {
		return vault.ErrUnavailable
	}
	if err := writeExclusive(identityFile, encrypted.Bytes()); err != nil {
		return err
	}
	if err := InitializeRecipient(storeDir, key.Recipient().String()); err != nil {
		_ = os.Remove(identityFile)
		return err
	}
	return nil
}

// InitializeRecipient creates a store for an existing identity's public recipient.
func InitializeRecipient(storeDir, recipient string) error {
	if _, err := agecrypto.ParseX25519Recipient(recipient); err != nil {
		return vault.ErrMalformed
	}
	storeDir = expand(storeDir)
	if storeDir == "" {
		return vault.ErrInvalidPath
	}
	if err := os.MkdirAll(storeDir, 0700); err != nil {
		return vault.ErrUnavailable
	}
	data, _ := json.MarshalIndent(manifest{Version: 1, Recipients: []string{recipient}}, "", "  ")
	return writeExclusive(filepath.Join(storeDir, manifestName), append(data, '\n'))
}

func writeExclusive(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".lazypass-init-")
	if err != nil {
		return vault.ErrUnavailable
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return vault.ErrUnavailable
	}
	if f.Close() != nil {
		return vault.ErrUnavailable
	}
	if err := os.Link(f.Name(), path); errors.Is(err, os.ErrExist) {
		return vault.ErrConflict
	} else if err != nil {
		return vault.ErrUnavailable
	}
	return nil
}

func (p *Provider) unlock(ctx context.Context) ([]agecrypto.Identity, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ctx.Err() != nil {
		return nil, vault.ErrUnavailable
	}
	if len(p.identities) > 0 {
		return p.identities, nil
	}
	info, err := os.Lstat(p.IdentityFile)
	if err != nil || !info.Mode().IsRegular() {
		return nil, vault.ErrLocked
	}
	if info.Size() > maxFileBytes {
		return nil, vault.ErrMalformed
	}
	data, err := os.ReadFile(p.IdentityFile)
	if err != nil {
		return nil, vault.ErrLocked
	}
	if !bytes.HasPrefix(data, []byte("age-encryption.org/v1\n")) {
		ids, err := agecrypto.ParseIdentities(bytes.NewReader(data))
		if err != nil || len(ids) == 0 {
			return nil, vault.ErrMalformed
		}
		p.identities = ids
		return ids, nil
	}
	if vault.IsNoninteractive(ctx) {
		return nil, vault.ErrPromptRequired
	}
	passphrase, err := vault.RequestPassphrase(ctx, p.Prompt)
	if err != nil {
		return nil, err
	}
	identity, err := agecrypto.NewScryptIdentity(passphrase)
	if err != nil {
		return nil, vault.ErrLocked
	}
	r, err := agecrypto.Decrypt(bytes.NewReader(data), identity)
	if err != nil {
		return nil, vault.ErrLocked
	}
	plain, err := io.ReadAll(io.LimitReader(r, maxFileBytes+1))
	if err != nil || len(plain) > maxFileBytes {
		return nil, vault.ErrLocked
	}
	defer clear(plain)
	ids, err := agecrypto.ParseIdentities(bytes.NewReader(plain))
	if err != nil || len(ids) == 0 {
		return nil, vault.ErrMalformed
	}
	p.identities = ids
	return ids, nil
}
