package bitwarden

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"lazypass/internal/vault"
)

func TestFolderTransferLive(t *testing.T) {
	dir := os.Getenv("LAZYPASS_BW_AUTH_DIR")
	if dir == "" {
		t.Skip("set live vault directory, server and password file")
	}
	p := New(dir, os.Getenv("LAZYPASS_BW_AUTH_SERVER"), func() (string, error) {
		data, err := os.ReadFile(os.Getenv("LAZYPASS_BW_AUTH_PASSWORD_FILE"))
		defer clear(data)
		return strings.TrimRight(string(data), "\r\n"), err
	}, nil)
	defer p.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	before, err := p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("initial snapshot failed")
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	prefix := "lazypass-folder-" + hex.EncodeToString(nonce[:])
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
					t.Error("item cleanup failed")
				}
			}
		}
		for _, f := range s.folders {
			if strings.HasPrefix(f.Name, prefix) {
				if _, err := p.run(cleanup, []string{"delete", "folder", f.ID}, nil); err != nil {
					t.Error("folder cleanup failed")
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
				t.Error("fixture remains")
			}
		}
		for _, f := range after.folders {
			if strings.HasPrefix(f.Name, prefix) {
				t.Error("fixture folder remains")
			}
		}
		for _, entry := range append(before.items, before.excluded...) {
			current, ok := remaining[entry.ID]
			if !ok || current.Name != entry.Name || current.FolderID != entry.FolderID || current.OrganizationID != entry.OrganizationID || strings.Join(current.CollectionIDs, ",") != strings.Join(entry.CollectionIDs, ",") {
				t.Error("original metadata changed")
			}
		}
	}()
	createFolder := func(name string) folder {
		t.Helper()
		f, err := decode[folder](p.submit(ctx, []string{"create", "folder"}, map[string]string{"name": name}))
		if err != nil || f.ID == "" {
			t.Fatal("folder fixture creation failed")
		}
		return f
	}
	parent := createFolder(prefix + "-source")
	child := createFolder(prefix + "-source/Child")
	if _, err := p.deletionSnapshot(ctx); err != nil {
		t.Fatal("folder sync failed")
	}
	if err := p.Create(ctx, vault.Entry{Path: vault.Path{"Folders", child.ID, prefix + "-item"}, Password: "public-folder-fixture", Notes: "preserved fixture"}); err != nil {
		t.Fatal("item fixture creation failed")
	}
	s, err := p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("item sync failed")
	}
	id := ""
	for _, entry := range s.items {
		if entry.Name == prefix+"-item" {
			id = entry.ID
		}
	}
	if id == "" {
		t.Fatal("fixture ID missing")
	}
	source := vault.Path{"Folders", parent.ID}
	if err := p.CheckFolderTransfer(ctx, source, vault.Path{"Folders"}, prefix+"-copy", false); err != nil {
		t.Fatal("copy preflight failed")
	}
	if err := p.TransferFolder(ctx, source, vault.Path{"Folders"}, prefix+"-copy", false); err != nil {
		t.Fatal("recursive copy failed")
	}
	s, err = p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("copy sync failed")
	}
	copyChild := ""
	for _, f := range s.folders {
		if f.Name == prefix+"-copy/Child" {
			copyChild = f.ID
		}
	}
	copyID := ""
	for _, entry := range s.items {
		if entry.FolderID == copyChild && entry.Name == prefix+"-item" {
			copyID = entry.ID
		}
	}
	if copyChild == "" || copyID == "" || copyID == id {
		t.Fatal("copy hierarchy or identity invalid")
	}
	entry, err := p.Read(ctx, vault.Path{"Folders", copyChild, copyID})
	if err != nil || entry.Password != "public-folder-fixture" || entry.Notes != "preserved fixture" {
		t.Fatal("copy fields changed")
	}
	if err := p.TransferFolder(ctx, source, vault.Path{"Folders"}, prefix+"-moved", true); err != nil {
		t.Fatal("recursive move failed")
	}
	s, err = p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("move sync failed")
	}
	found := false
	for _, f := range s.folders {
		if f.ID == child.ID && f.Name == prefix+"-moved/Child" {
			found = true
		}
	}
	if !found {
		t.Fatal("folder identity not preserved")
	}
	if _, err := p.Read(ctx, vault.Path{"Folders", child.ID, id}); err != nil {
		t.Fatal("move changed item identity")
	}
}
