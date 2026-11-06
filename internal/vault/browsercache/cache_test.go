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

func TestCacheScopeSeparatesAccounts(t *testing.T) {
	base := &Store{Path: filepath.Join(t.TempDir(), "base.json")}
	a, b := base.Scoped("server/account-a"), base.Scoped("server/account-b")
	if a.Path == b.Path || a.Path != base.Scoped("server/account-a").Path {
		t.Fatal("unstable account isolation")
	}
	if err := a.Save(nil, []vault.Node{{Name: "a", Path: vault.Path{"a"}, Kind: vault.EntryNode}}); err != nil {
		t.Fatal(err)
	}
	locations, err := b.Load()
	if err != nil || len(locations) != 0 {
		t.Fatal("account b read account a cache")
	}
}
