package bitwarden

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/otp"
	vaultui "lazypass/internal/tui/vault"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

// This opt-in test edits only newly created, uniquely identified public fixtures.
func TestAuthenticatorLive(t *testing.T) {
	dir := os.Getenv("LAZYPASS_BW_AUTH_DIR")
	if dir == "" {
		t.Skip("set authenticator test directory, server and password file")
	}
	p := New(dir, os.Getenv("LAZYPASS_BW_AUTH_SERVER"), func() (string, error) {
		data, err := os.ReadFile(os.Getenv("LAZYPASS_BW_AUTH_PASSWORD_FILE"))
		defer clear(data)
		return strings.TrimRight(string(data), "\r\n"), err
	}, nil)
	defer p.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	before, err := p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("initial synced snapshot failed")
	}
	var token [12]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	name := "lazypass-totp-" + hex.EncodeToString(token[:])
	created := make(map[string]bool)
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Minute)
		defer stop()
		s, err := p.deletionSnapshot(cleanup)
		if err != nil {
			t.Error("cleanup snapshot failed")
			return
		}
		for _, item := range append(s.items, s.excluded...) {
			if created[item.ID] || item.Name == name+"-personal" || item.Name == name+"-organization" {
				if _, err := p.run(cleanup, []string{"delete", "item", item.ID}, nil); err != nil {
					t.Error("fixture cleanup failed")
				}
			}
		}
		after, err := p.deletionSnapshot(cleanup)
		if err != nil {
			t.Error("cleanup verification failed")
			return
		}
		remaining := make(map[string]item)
		for _, item := range append(after.items, after.excluded...) {
			remaining[item.ID] = item
			if created[item.ID] || item.Name == name+"-personal" || item.Name == name+"-organization" {
				t.Error("active fixture remains")
			}
		}
		for _, original := range append(before.items, before.excluded...) {
			current, ok := remaining[original.ID]
			if !ok || current.Name != original.Name || current.OrganizationID != original.OrganizationID || current.FolderID != original.FolderID || strings.Join(current.CollectionIDs, ",") != strings.Join(original.CollectionIDs, ",") {
				t.Error("original entry metadata changed")
			}
		}
	}()
	for _, organization := range []bool{false, true} {
		label := "personal"
		path := vault.Path{"Folders", noFolderID, name + "-personal"}
		if organization {
			label = "organization"
			if len(before.collections) == 0 {
				t.Error("organization qualification needs an existing accessible collection")
				continue
			}
			group := before.collections[0]
			path = vault.Path{"Organizations", group.OrganizationID, group.ID, name + "-organization"}
		}
		t.Run(label, func(t *testing.T) {
			const first = "otpauth://totp/PublicFixture?secret=JBSWY3DPEHPK3PXP"
			const second = "otpauth://totp/PublicFixture?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&digits=8&algorithm=SHA256&period=60"
			importer := service.Service{Provider: p}
			results := importer.ImportEntries(ctx, []vault.Entry{{Path: path, Authenticator: first}}, nil)
			if len(results) != 1 || !results[0].Created {
				t.Fatal("fixture batch creation failed")
			}
			s, err := p.deletionSnapshot(ctx)
			if err != nil {
				t.Fatal("fixture sync failed")
			}
			id := ""
			for _, item := range append(s.items, s.excluded...) {
				if item.Name == name+"-"+label {
					id = item.ID
					created[id] = true
				}
			}
			if id == "" {
				t.Fatal("fixture ID missing")
			}
			path[len(path)-1] = id
			results = importer.ImportEntries(ctx, []vault.Entry{{Path: path, Authenticator: second}}, nil)
			if results[0].Created || results[0].Uncertain {
				t.Fatal("existing fixture was overwritten or incorrectly classified")
			}
			for _, expected := range []string{first, second, ""} {
				entry, err := p.Read(ctx, path)
				if err != nil || entry.Password != "" || entry.Username != "" || entry.AuthenticatorUnavailable {
					t.Fatal("authenticator-only fixture read failed")
				}
				if expected != first {
					entry.Authenticator = expected
					entry.AuthenticatorChanged = true
					if err := p.Write(ctx, entry); err != nil {
						t.Fatal("fixture authenticator edit failed")
					}
					entry, err = p.Read(ctx, path)
				}
				if err != nil || entry.Authenticator != expected {
					t.Fatal("authenticator round trip mismatch")
				}
				if expected != "" {
					key, err := otp.Parse(expected)
					if err != nil {
						t.Fatal("public fixture parse failed")
					}
					start := time.Now()
					code, err := p.AuthenticatorCode(ctx, path)
					end := time.Now()
					startCode, _, _ := key.Code(start)
					endCode, _, _ := key.Code(end)
					if err != nil || (code != startCode && code != endCode) {
						t.Fatal("native code did not match public fixture")
					}
					session, err := (service.Service{Provider: p}).LoadAuthenticator(vault.WithoutPrompt(ctx), path)
					if err != nil {
						t.Fatal("inline native session failed")
					}
					inline, remaining := session.Current(time.Now())
					if inline == "" || remaining <= 0 || session.Expiry.Unix()%int64(key.Period()/time.Second) != 0 {
						t.Fatal("inline native timing invalid")
					}
					entry.AuthenticatorChanged = false
					entry.Notes = "Public authenticator preservation fixture"
					if err := p.Write(ctx, entry); err != nil {
						t.Fatal("unrelated edit failed")
					}
				} else if _, err := p.AuthenticatorCode(ctx, path); err == nil {
					t.Fatal("removed authenticator still generated a code")
				}
			}
		})
	}
	t.Run("marked deletion through UI", func(t *testing.T) {
		snapshot, err := p.deletionSnapshot(ctx)
		if err != nil {
			t.Fatal("fixture listing failed")
		}
		var nodes []vault.Node
		for _, entry := range append(snapshot.items, snapshot.excluded...) {
			if created[entry.ID] {
				nodes = append(nodes, vault.Node{Name: entry.Name, Path: vault.Path{"Folders", noFolderID, entry.ID}, Kind: vault.EntryNode, Shared: entry.OrganizationID != ""})
			}
		}
		if len(nodes) != 2 {
			t.Fatal("expected two active disposable fixtures")
		}
		provider := &markedFixtureProvider{Provider: p, nodes: nodes}
		view := vaultui.New(vaultui.Dependencies{Service: service.Service{Provider: provider}, RunPrompted: func(work func(context.Context) error) error { return work(ctx) }})
		view.Refresh()
		for _, r := range "mjMd" {
			view.HandleKey(tcell.NewEventKey(tcell.KeyRune, r, 0))
		}
		if !view.IsConfirmingDelete() {
			t.Fatal("marked confirmation not opened")
		}
		view.PasteConfirmation("DELETE 2")
		view.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
		after, err := p.deletionSnapshot(ctx)
		if err != nil {
			t.Fatal("marked deletion verification failed")
		}
		for _, entry := range append(after.items, after.excluded...) {
			if created[entry.ID] {
				t.Fatal("marked fixture remains active")
			}
		}
	})
}

// Restrict UI listing to captured fixture IDs so no existing entry can be marked.
type markedFixtureProvider struct {
	*Provider
	nodes []vault.Node
}

func (p *markedFixtureProvider) List(context.Context, vault.Path) ([]vault.Node, error) {
	return p.nodes, nil
}
