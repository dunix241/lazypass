package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"lazypass/internal/vault"
)

func TestRecordsPersistAndPreserveSameNamedEntry(t *testing.T) {
	root := t.TempDir()
	for _, path := range []vault.Path{{"same"}, {"same", "child"}} {
		if err := Write(root, path, vault.CardEntry); err != nil {
			t.Fatal(err)
		}
	}
	nodes := []vault.Node{{Path: vault.Path{"same"}, Kind: vault.EntryNode}, {Path: vault.Path{"same", "child"}, Kind: vault.EntryNode}}
	Decorate(root, nodes)
	if !nodes[0].TypeKnown || nodes[0].EntryType != vault.CardEntry {
		t.Fatal("type not restored")
	}
	if err := Remove(root, vault.Path{"same"}, true); err != nil {
		t.Fatal(err)
	}
	nodes[0].TypeKnown = false
	nodes[1].TypeKnown = false
	Decorate(root, nodes)
	if !nodes[0].TypeKnown || nodes[1].TypeKnown {
		t.Fatal("recursive metadata deletion affected wrong record")
	}
}

func TestInvalidMetadataIsIgnoredAndSymlinkRejected(t *testing.T) {
	root := t.TempDir()
	path := vault.Path{"entry"}
	if err := Write(root, path, vault.NoteEntry); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, RecordPath(path)), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	nodes := []vault.Node{{Path: path, Kind: vault.EntryNode}}
	Decorate(root, nodes)
	if nodes[0].TypeKnown {
		t.Fatal("invalid record used")
	}
	outside := t.TempDir()
	store := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(store, Directory)); err != nil {
		t.Skip(err)
	}
	if err := Write(store, path, vault.CardEntry); err == nil {
		t.Fatal("metadata symlink accepted")
	}
}

func TestDeletedAndCorruptRecordsClearKnownTypes(t *testing.T) {
	root := t.TempDir()
	path := vault.Path{"entry"}
	if err := Write(root, path, vault.IdentityEntry); err != nil {
		t.Fatal(err)
	}
	nodes := []vault.Node{{Path: path, Kind: vault.EntryNode}}
	Decorate(root, nodes)
	if !nodes[0].TypeKnown {
		t.Fatal("record not applied")
	}
	if err := os.Remove(filepath.Join(root, RecordPath(path))); err != nil {
		t.Fatal(err)
	}
	Decorate(root, nodes)
	if nodes[0].TypeKnown {
		t.Fatal("deleted metadata retained known type")
	}
	if err := Write(root, path, vault.CardEntry); err != nil {
		t.Fatal(err)
	}
	Decorate(root, nodes)
	if err := os.WriteFile(filepath.Join(root, RecordPath(path)), []byte(`{"version":1,"path":"other","type":2}`), 0600); err != nil {
		t.Fatal(err)
	}
	Decorate(root, nodes)
	if nodes[0].TypeKnown {
		t.Fatal("mismatched record retained known type")
	}
}

func TestUnchangedMetadataIsNotRewritten(t *testing.T) {
	root := t.TempDir()
	path := vault.Path{"entry"}
	if err := Write(root, path, vault.NoteEntry); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, RecordPath(path))
	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(root, path, vault.NoteEntry); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("unchanged record replaced")
	}
}
