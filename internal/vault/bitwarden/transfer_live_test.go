package bitwarden

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"lazypass/internal/vault"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTransferLive(t *testing.T) {
	dir := os.Getenv("LAZYPASS_BW_AUTH_DIR")
	if dir == "" {
		t.Skip("set live vault test directory, server and password file")
	}
	p := New(dir, os.Getenv("LAZYPASS_BW_AUTH_SERVER"), func() (string, error) {
		data, err := os.ReadFile(os.Getenv("LAZYPASS_BW_AUTH_PASSWORD_FILE"))
		defer clear(data)
		return strings.TrimRight(string(data), "\r\n"), err
	}, nil)
	defer p.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	before, err := p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("initial snapshot failed")
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	prefix := "lazypass-transfer-" + hex.EncodeToString(nonce[:])
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Minute)
		defer stop()
		s, err := p.deletionSnapshot(cleanup)
		if err != nil {
			t.Error("cleanup snapshot failed")
			return
		}
		for _, entry := range append(s.items, s.excluded...) {
			if strings.HasPrefix(entry.Name, prefix) {
				if _, err := p.run(cleanup, []string{"delete", "item", entry.ID}, nil); err != nil {
					t.Error("fixture cleanup failed")
				}
			}
		}
		after, err := p.deletionSnapshot(cleanup)
		if err != nil {
			t.Error("cleanup verification failed")
			return
		}
		remaining := map[string]item{}
		for _, entry := range append(after.items, after.excluded...) {
			remaining[entry.ID] = entry
			if strings.HasPrefix(entry.Name, prefix) {
				t.Error("active fixture remains")
			}
		}
		for _, original := range append(before.items, before.excluded...) {
			current, ok := remaining[original.ID]
			if !ok || current.Name != original.Name || current.FolderID != original.FolderID || current.OrganizationID != original.OrganizationID || strings.Join(current.CollectionIDs, ",") != strings.Join(original.CollectionIDs, ",") {
				t.Error("original metadata changed")
			}
		}
	}()
	for _, org := range []bool{false, true} {
		parent := vault.Path{"Folders", noFolderID}
		label := "personal"
		if org {
			if len(before.collections) == 0 {
				t.Error("organization fixture needs accessible collection")
				continue
			}
			c := before.collections[0]
			parent = vault.Path{"Organizations", c.OrganizationID, c.ID}
			label = "organization"
		}
		t.Run(label, func(t *testing.T) {
			name := prefix + "-" + label
			if err := p.Create(ctx, vault.Entry{Path: append(append(vault.Path(nil), parent...), name), Password: "public-transfer-fixture", Authenticator: "otpauth://totp/Fixture?secret=JBSWY3DPEHPK3PXP", Notes: "Public transfer fixture"}); err != nil {
				t.Fatal("fixture creation failed")
			}
			s, err := p.deletionSnapshot(ctx)
			if err != nil {
				t.Fatal("fixture snapshot failed")
			}
			id := ""
			for _, entry := range append(s.items, s.excluded...) {
				if entry.Name == name {
					id = entry.ID
				}
			}
			if id == "" {
				t.Fatal("fixture ID missing")
			}
			source := append(append(vault.Path(nil), parent...), id)
			attachmentPath := filepath.Join(t.TempDir(), "public-fixture.txt")
			if err := os.WriteFile(attachmentPath, []byte("public attachment transfer fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			output, err := p.run(ctx, []string{"create", "attachment", "--file", attachmentPath, "--itemid", id}, nil)
			clear(output)
			if err != nil {
				t.Fatal("attachment fixture creation failed")
			}
			if err := p.Rename(ctx, vault.Node{Path: source, Kind: vault.EntryNode}, name+"-renamed"); err != nil {
				t.Fatal("native rename failed")
			}
			if _, err := p.deletionSnapshot(ctx); err != nil {
				t.Fatal("rename sync failed")
			}
			loadedRenamed, err := p.Read(ctx, source)
			if err != nil || loadedRenamed.Password != "public-transfer-fixture" || loadedRenamed.Authenticator == "" {
				t.Fatal("rename changed identity or fields")
			}
			if err := p.Rename(ctx, vault.Node{Path: source, Kind: vault.EntryNode}, name); err != nil {
				t.Fatal("rename restoration failed")
			}
			if _, err := p.deletionSnapshot(ctx); err != nil {
				t.Fatal("rename restoration sync failed")
			}
			if err := p.CheckTransfer(ctx, source, parent, name+"-copy", false); err != nil {
				t.Fatal("copy preflight failed")
			}
			if err := p.Transfer(ctx, source, parent, name+"-copy", false); err != nil {
				t.Fatal("native copy failed:", err)
			}
			s, err = p.deletionSnapshot(ctx)
			if err != nil {
				t.Fatal("copy sync failed")
			}
			copyID := ""
			for _, entry := range append(s.items, s.excluded...) {
				if entry.Name == name+"-copy" {
					copyID = entry.ID
				}
			}
			if copyID == "" || copyID == id {
				t.Fatal("clone identity invalid")
			}
			loaded, err := p.Read(ctx, append(append(vault.Path(nil), parent...), copyID))
			if err != nil || loaded.Password != "public-transfer-fixture" || loaded.Authenticator == "" || loaded.Notes != "Public transfer fixture" {
				t.Fatal("clone fields changed")
			}
			if !org && len(before.folders) > 0 {
				target := vault.Path{"Folders", before.folders[0].ID}
				if err := p.Transfer(ctx, source, target, name, true); err != nil {
					t.Fatal("personal move failed")
				}
				if _, err := p.deletionSnapshot(ctx); err != nil {
					t.Fatal("move sync failed")
				}
				if _, err := p.Read(ctx, append(target, id)); err != nil {
					t.Fatal("move did not preserve ID")
				}
			}
			if org {
				var target *collection
				for i := range before.collections {
					candidate := &before.collections[i]
					if candidate.OrganizationID == parent[1] && candidate.ID != parent[2] && !deniedCollection(*candidate) {
						target = candidate
						break
					}
				}
				if target == nil {
					t.Error("membership move needs two writable collections")
					return
				}
				destination := vault.Path{"Organizations", parent[1], target.ID}
				if err := p.Transfer(ctx, source, destination, name, true); err != nil {
					t.Fatal("collection membership move failed")
				}
				if _, err := p.deletionSnapshot(ctx); err != nil {
					t.Fatal("membership sync failed")
				}
				moved := append(append(vault.Path(nil), destination...), id)
				loaded, err := p.Read(ctx, moved)
				if err != nil || loaded.Password != "public-transfer-fixture" {
					t.Fatal("membership move lost identity or fields")
				}
				if err := p.Transfer(ctx, moved, parent, name, true); err != nil {
					t.Fatal("membership move restoration failed")
				}
			}
		})
	}
}
