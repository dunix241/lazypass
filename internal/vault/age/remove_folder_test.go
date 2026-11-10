package age

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"lazypass/internal/vault"
)

func TestRemoveFolderMovesEntireContentsToUnorganized(t *testing.T) {
	dir := t.TempDir()
	store, identity := filepath.Join(dir, "store"), filepath.Join(dir, "identity.age")
	if err := Initialize(store, identity, "fixture"); err != nil {
		t.Fatal(err)
	}
	p := New(store, identity, func() (string, error) { return "fixture", nil })
	ctx := context.Background()
	for _, path := range []vault.Path{{"Work", "Services", "api"}, {"Work", "Services", "Production", "admin"}} {
		if err := p.Create(ctx, vault.Entry{Path: path, Type: vault.NoteEntry, Notes: "public fixture"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.RemoveFolder(ctx, vault.Path{"Work", "Services"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(store, "Work", "Services")); !os.IsNotExist(err) {
		t.Fatal("removed folder remains")
	}
	for _, path := range []vault.Path{{"Unorganized", "api"}, {"Unorganized", "Production", "admin"}} {
		entry, err := p.Read(ctx, path)
		if err != nil || entry.Notes != "public fixture" {
			t.Fatalf("preserved entry missing: %v", err)
		}
	}
	nodes, err := p.List(ctx, vault.Path{"Unorganized"})
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		if node.Kind == vault.EntryNode && (!node.TypeKnown || node.EntryType != vault.NoteEntry) {
			t.Fatal("type metadata not moved")
		}
	}
}
