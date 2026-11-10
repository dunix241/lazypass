package age

import (
	"context"
	"errors"
	agecrypto "filippo.io/age"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

func TestProtectedVaultLifecycleAndRecovery(t *testing.T) {
	home := t.TempDir()
	store, identity := filepath.Join(home, "store"), filepath.Join(home, "identity.age")
	if err := Initialize(store, identity, "public-test-passphrase"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(identity)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "AGE-SECRET-KEY-") {
		t.Fatal("plaintext identity persisted")
	}
	prompts := 0
	p := New(store, identity, func() (string, error) { prompts++; return "public-test-passphrase", nil })
	ctx := context.Background()
	path := vault.Path{"hello", "world"}
	entry := vault.Entry{Path: path, Password: "fixture-secret", Username: "user", Notes: "note"}
	if err := p.Create(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if prompts != 0 {
		t.Fatal("encryption required unlocking")
	}
	if err := p.Create(ctx, entry); !errors.Is(err, vault.ErrConflict) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := p.Read(vault.WithoutPrompt(ctx), path); !errors.Is(err, vault.ErrPromptRequired) {
		t.Fatalf("locked probe: %v", err)
	}
	got, err := p.Read(ctx, path)
	if err != nil || got.Password != entry.Password || got.Username != "user" {
		t.Fatalf("read: %+v %v", got, err)
	}
	if _, err := p.Read(vault.WithoutPrompt(ctx), path); err != nil || prompts != 1 {
		t.Fatal("session cache missing")
	}
	p.Lock()
	p.Prompt = func() (string, error) { return "wrong", nil }
	if _, err := p.Read(ctx, path); !errors.Is(err, vault.ErrLocked) {
		t.Fatal("wrong passphrase accepted")
	}
	restored := New(store, identity, func() (string, error) { return "public-test-passphrase", nil })
	got, err = restored.Read(ctx, path)
	if err != nil || got.Password != entry.Password {
		t.Fatal("restart recovery failed")
	}
	got.Password = "updated"
	got.Authenticator = "otpauth://totp/Fixture?secret=JBSWY3DPEHPK3PXP"
	if err := restored.Write(ctx, got); err != nil {
		t.Fatal(err)
	}
	got, err = restored.Read(ctx, path)
	if err != nil || got.Authenticator == "" {
		t.Fatal("encrypted authenticator did not round trip")
	}
	copied := ""
	s := service.Service{Provider: restored, Copy: func(value string) error { copied = value; return nil }}
	if err := s.CopyAuthenticator(ctx, path); err != nil || len(copied) != 6 {
		t.Fatal("age authenticator copy failed")
	}
	bulkPath := vault.Path{"hello", "bulk-fixture"}
	results := s.ImportEntries(ctx, []vault.Entry{{Path: bulkPath, Authenticator: got.Authenticator}}, nil)
	if len(results) != 1 || !results[0].Created {
		t.Fatal("age bulk import failed")
	}
	results = s.ImportEntries(ctx, []vault.Entry{{Path: bulkPath, Authenticator: got.Authenticator}}, nil)
	if results[0].Created || results[0].Uncertain {
		t.Fatal("age bulk conflict was not preserved")
	}
	bulk, err := restored.Read(ctx, bulkPath)
	if err != nil || bulk.Password != "" || bulk.Authenticator != got.Authenticator {
		t.Fatal("age imported entry did not reopen")
	}
	if err := restored.Delete(ctx, bulkPath); err != nil {
		t.Fatal(err)
	}
	got.Authenticator = ""
	if err := restored.Write(ctx, got); err != nil {
		t.Fatal(err)
	}
	got, err = restored.Read(ctx, path)
	if err != nil || got.Authenticator != "" {
		t.Fatal("removed authenticator survived")
	}
	if err := restored.Delete(ctx, path); err != nil {
		t.Fatal(err)
	}
	nodes, err := restored.List(ctx, vault.Path{"hello"})
	if err != nil || len(nodes) != 0 {
		t.Fatal("parent folder not preserved")
	}
	if err := restored.Create(ctx, vault.Entry{Path: vault.Path{"hello", "nested", "entry"}, Password: "fixture"}); err != nil {
		t.Fatal(err)
	}
	if err := restored.DeleteFolder(ctx, vault.Path{"hello"}); err != nil {
		t.Fatal(err)
	}
	if err := restored.DeleteFolder(ctx, nil); !errors.Is(err, vault.ErrInvalidPath) {
		t.Fatal("root deletion allowed")
	}
	if err := Initialize(store, identity, "another"); !errors.Is(err, vault.ErrConflict) {
		t.Fatal("initialization overwrote vault")
	}
}

func TestConcurrentCreateAndContainment(t *testing.T) {
	home := t.TempDir()
	store, identity := filepath.Join(home, "store"), filepath.Join(home, "identity.age")
	if err := Initialize(store, identity, "fixture"); err != nil {
		t.Fatal(err)
	}
	p := New(store, identity, nil)
	ctx := context.Background()
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- p.Create(ctx, vault.Entry{Path: vault.Path{"entry"}, Password: "fixture"})
		}()
	}
	wg.Wait()
	a, b := <-results, <-results
	if (a != nil || !errors.Is(b, vault.ErrConflict)) && (b != nil || !errors.Is(a, vault.ErrConflict)) {
		t.Fatalf("concurrent create: %v %v", a, b)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(store, "link")); err == nil {
		if err := p.Create(ctx, vault.Entry{Path: vault.Path{"link", "escape"}, Password: "fixture"}); !errors.Is(err, vault.ErrInvalidPath) {
			t.Fatalf("symlink traversal: %v", err)
		}
	}
	for _, path := range []vault.Path{{".."}, {manifestName}, {"entry.age"}} {
		if err := p.Create(ctx, vault.Entry{Path: path, Password: "fixture"}); !errors.Is(err, vault.ErrInvalidPath) {
			t.Fatalf("invalid path accepted: %v", path)
		}
	}
	nodes, err := p.List(ctx, nil)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("metadata: %v %v", nodes, err)
	}
}

func TestRebuildRestoresIdentityTypeWithoutEditing(t *testing.T) {
	home := t.TempDir()
	store, identity := filepath.Join(home, "store"), filepath.Join(home, "identity.age")
	if err := Initialize(store, identity, "fixture"); err != nil {
		t.Fatal(err)
	}
	p := New(store, identity, func() (string, error) { return "fixture", nil })
	path := vault.Path{"identity"}
	if err := p.Create(context.Background(), vault.Entry{Path: path, Type: vault.IdentityEntry, Fields: map[string]string{"firstName": "fixture"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(store, ".lazypass")); err != nil {
		t.Fatal(err)
	}
	result, err := (service.Service{Provider: p}).RebuildMetadata(context.Background())
	if err != nil || result.Indexed != 1 || result.Failed != 0 {
		t.Fatalf("rebuild: %+v %v", result, err)
	}
	nodes, err := p.List(context.Background(), nil)
	if err != nil || len(nodes) != 1 || !nodes[0].TypeKnown || nodes[0].EntryType != vault.IdentityEntry {
		t.Fatalf("identity type missing after rebuild: %+v %v", nodes, err)
	}
}

func TestRejectIdentityInsideStore(t *testing.T) {
	store := t.TempDir()
	if err := Initialize(store, filepath.Join(store, "identity.age"), "fixture"); !errors.Is(err, vault.ErrInvalidPath) {
		t.Fatal("identity stored inside vault")
	}
}

func TestTypedIconMetadataSurvivesProviderRestart(t *testing.T) {
	home := t.TempDir()
	store, identity := filepath.Join(home, "store"), filepath.Join(home, "identity.age")
	if err := Initialize(store, identity, "fixture"); err != nil {
		t.Fatal(err)
	}
	p := New(store, identity, nil)
	if err := p.Create(context.Background(), vault.Entry{Type: vault.CardEntry, Path: vault.Path{"card"}, Fields: map[string]string{"number": "fixture"}}); err != nil {
		t.Fatal(err)
	}
	fresh := New(store, identity, nil)
	nodes, err := fresh.List(context.Background(), nil)
	if err != nil || len(nodes) != 1 || !nodes[0].TypeKnown || nodes[0].EntryType != vault.CardEntry {
		t.Fatalf("persistent type missing: %v %v", nodes, err)
	}
}

func TestExistingIdentityAndMissingKey(t *testing.T) {
	home := t.TempDir()
	key, err := agecrypto.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	identity := filepath.Join(home, "identity.txt")
	if err := os.WriteFile(identity, []byte("# existing identity\n"+key.String()+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(home, "store")
	if err := InitializeRecipient(store, key.Recipient().String()); err != nil {
		t.Fatal(err)
	}
	p := New(store, identity, nil)
	path := vault.Path{"entry"}
	ctx := context.Background()
	if err := p.Create(ctx, vault.Entry{Path: path, Password: "fixture"}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Read(vault.WithoutPrompt(ctx), path); err != nil {
		t.Fatal(err)
	}
	p.Lock()
	if err := os.Remove(identity); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Read(ctx, path); !errors.Is(err, vault.ErrLocked) {
		t.Fatal("missing identity not reported as locked")
	}
}
