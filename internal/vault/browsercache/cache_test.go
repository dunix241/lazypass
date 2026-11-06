package browsercache

import (
	"os"
	"path/filepath"
	"testing"

	"lazypass/internal/vault"
)

func TestCacheAcrossStoreInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser.json")
	s := &Store{Path: path}
	nodes := []vault.Node{{Name: "entry", Path: vault.Path{"Personal", "entry"}, Kind: vault.EntryNode}}
	if err := s.Save(vault.Path{"Personal"}, nodes); err != nil {
		t.Fatal(err)
	}
	loaded, err := (&Store{Path: path}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded["Personal"]) != 1 {
		t.Fatal("cache did not survive new instance")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("cache permissions")
	}
}

func TestCorruptCacheRecoversOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "browser.json")
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Store{Path: path}
	if _, err := s.Load(); err == nil {
		t.Fatal("corruption accepted")
	}
	if err := s.Save(nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err != nil {
		t.Fatal(err)
	}
}
