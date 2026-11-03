package bitwarden

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

// Opt-in tests create only public fixtures in a uniquely named folder.
func TestLiveProvider(t *testing.T) {
	credentialFile := os.Getenv("LAZYPASS_BW_TEST_PASSWORD_FILE")
	if credentialFile == "" {
		t.Skip("set LAZYPASS_BW_TEST_PASSWORD_FILE and LAZYPASS_BW_TEST_DATA_DIR")
	}
	dataDir := os.Getenv("LAZYPASS_BW_TEST_DATA_DIR")
	if dataDir == "" {
		t.Fatal("LAZYPASS_BW_TEST_DATA_DIR required")
	}
	credential, err := os.ReadFile(credentialFile)
	if err != nil {
		t.Fatal("could not read test credential file")
	}
	t.Cleanup(func() { clear(credential) })
	p := New(dataDir, os.Getenv("LAZYPASS_BW_TEST_SERVER"), func() (string, error) {
		return strings.TrimSuffix(strings.TrimSuffix(string(credential), "\n"), "\r"), nil
	}, nil)
	t.Cleanup(p.Lock)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	folderName := "lazypass-integration-" + time.Now().UTC().Format("20060102T150405.000000000")
	folderPath := vault.Path{folderName}
	s := service.Service{Provider: p}
	if err := p.CreateFolder(ctx, folderPath); err != nil {
		t.Fatalf("create fixture folder: %v", err)
	}
	t.Log("created isolated fixture folder:", folderName)
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 3*time.Minute)
		defer stop()
		if err := p.DeleteFolder(cleanup, folderPath); err != nil {
			t.Errorf("fixture cleanup failed for %s: %v", folderName, err)
		}
	})
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(private, "public-integration-fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range vault.AllEntryTypes() {
		t.Run(kind.String(), func(t *testing.T) {
			entry := vault.Entry{Type: kind, Path: vault.Path{folderName, kind.String()}, Password: "public-password-fixture", Username: "fixture-user", URL: "https://example.invalid", Notes: "public notes\nsecond line"}
			switch kind {
			case vault.CardEntry:
				entry.Fields = map[string]string{"cardholderName": "Public Fixture", "brand": "Visa", "number": "4111111111111111", "expMonth": "12", "expYear": "2030", "code": "123"}
			case vault.IdentityEntry:
				entry.Fields = map[string]string{"firstName": "Public", "lastName": "Fixture", "email": "fixture@example.invalid"}
			case vault.SSHKeyEntry:
				entry.Fields = map[string]string{"privateKey": string(pem.EncodeToMemory(block))}
			}
			if err := s.Create(ctx, entry); err != nil {
				t.Fatalf("create %s: %v", kind, err)
			}
			got, err := s.Show(ctx, entry.Path)
			if err != nil {
				t.Fatalf("read %s: %v", kind, err)
			}
			if got.Type != kind || got.Notes != entry.Notes {
				t.Fatal("type or notes changed")
			}
			if kind == vault.LoginEntry && (got.Password != entry.Password || got.Username != entry.Username || got.URL != entry.URL) {
				t.Fatal("login fields changed")
			}
			for key, value := range entry.Fields {
				if got.Fields[key] != value && key != "privateKey" {
					t.Fatalf("field %s changed", key)
				}
			}
			got.Notes = "updated public notes"
			if kind == vault.LoginEntry {
				got.Password = "updated-public-password"
			}
			if err := s.Store(ctx, got); err != nil {
				t.Fatalf("edit %s: %v", kind, err)
			}
			updated, err := s.Show(ctx, entry.Path)
			if err != nil || updated.Notes != got.Notes || updated.Type != kind {
				t.Fatalf("edit round-trip: %v", err)
			}
			if err := s.Create(ctx, entry); err != vault.ErrConflict {
				t.Fatalf("duplicate check: %v", err)
			}
			if err := s.Delete(ctx, vault.Node{Path: entry.Path, Kind: vault.EntryNode}); err != nil {
				t.Fatalf("delete: %v", err)
			}
			t.Log("create/read/edit/duplicate/delete passed")
		})
	}
	if _, err := p.Sync(ctx); err != nil {
		t.Fatalf("sync: %v", err)
	}
}
