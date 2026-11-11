package age

import (
	"context"
	"lazypass/internal/vault"
	"os"
	"path/filepath"
	"testing"
)

func TestTransferEncryptedTypedEntryWithoutUnlock(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	identity := filepath.Join(dir, "identity")
	if err := Initialize(store, identity, "fixture-passphrase"); err != nil {
		t.Fatal(err)
	}
	p := New(store, identity, func() (string, error) { t.Fatal("transfer requested unlock"); return "", nil })
	ctx := context.Background()
	if err := p.Create(ctx, vault.Entry{Path: vault.Path{"source"}, Type: vault.NoteEntry, Notes: "public fixture\nwith tail"}); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(store, "source.age"))
	if err != nil {
		t.Fatal(err)
	}
	if err := p.CreateFolder(ctx, vault.Path{"target"}); err != nil {
		t.Fatal(err)
	}
	if err := p.Transfer(ctx, vault.Path{"source"}, vault.Path{"target"}, "copy", false); err != nil {
		t.Fatal(err)
	}
	if err := p.Transfer(ctx, vault.Path{"source"}, nil, "moved", true); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"moved.age", "target/copy.age"} {
		got, err := os.ReadFile(filepath.Join(store, path))
		if err != nil || string(got) != string(original) {
			t.Fatal("ciphertext changed")
		}
	}
	nodes, err := p.List(ctx, vault.Path{"target"})
	if err != nil || len(nodes) != 1 || nodes[0].EntryType != vault.NoteEntry {
		t.Fatal("metadata not transferred")
	}
	if err := p.CreateFolder(ctx, vault.Path{"target", "empty"}); err != nil {
		t.Fatal(err)
	}
	if err := p.TransferFolder(ctx, vault.Path{"target"}, nil, "folder-copy", false); err != nil {
		t.Fatal(err)
	}
	if err := p.TransferFolder(ctx, vault.Path{"target"}, nil, "folder-moved", true); err != nil {
		t.Fatal(err)
	}
	for _, folder := range []string{"folder-copy", "folder-moved"} {
		nodes, err := p.List(ctx, vault.Path{folder})
		if err != nil || len(nodes) != 2 {
			t.Fatal("recursive folder contents missing")
		}
		if nodes[1].EntryType != vault.NoteEntry || !nodes[1].TypeKnown {
			t.Fatal("recursive metadata lost")
		}
	}
}
