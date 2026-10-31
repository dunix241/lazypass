package vault

import (
	"testing"

	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
)

func TestRecursiveSearchFindsAndCopiesNestedEntry(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"":            {{Name: "hihi", Path: vault.Path{"hihi"}, Kind: vault.FolderNode}},
		"hihi":        {{Name: "test", Path: vault.Path{"hihi", "test"}, Kind: vault.EntryNode}, {Name: "nested", Path: vault.Path{"hihi", "nested"}, Kind: vault.FolderNode}},
		"hihi/nested": {{Name: "test", Path: vault.Path{"hihi", "nested", "test"}, Kind: vault.EntryNode}},
	}, read: map[string]vault.Entry{"hihi/test": {Password: "fixture"}}}
	var copied string
	v := New(Dependencies{Service: service.Service{Provider: p, Copy: func(value string) error { copied = value; return nil }}})
	v.Refresh()
	for _, r := range "/test" {
		key(v, tcell.KeyRune, r)
	}
	nodes := v.filteredVaultNodes()
	if len(nodes) != 2 || nodes[0].Path.String() != "hihi/test" || v.searchName(nodes[1]) != "hihi/nested/test" {
		t.Fatalf("results=%v", nodes)
	}
	key(v, tcell.KeyEnter, 0)
	if copied != "fixture" {
		t.Fatal("nested entry did not copy")
	}
	key(v, tcell.KeyEscape, 0)
	if v.IsFiltering() || len(v.vaultDisplayNodes()) != 1 {
		t.Fatal("search did not restore browser")
	}
	v.vaultPath = vault.Path{"hihi"}
	v.Refresh()
	for _, r := range "/test" {
		key(v, tcell.KeyRune, r)
	}
	if v.searchName(v.filteredVaultNodes()[0]) != "test" {
		t.Fatal("search labels not relative to current folder")
	}
}

func TestRecursiveSearchMatchesFolderPathsAndOpensFolder(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "parent", Path: vault.Path{"parent"}, Kind: vault.FolderNode}}, "parent": {{Name: "child", Path: vault.Path{"parent", "child"}, Kind: vault.FolderNode}}}}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.Refresh()
	for _, r := range "/parent/child" {
		key(v, tcell.KeyRune, r)
	}
	if len(v.filteredVaultNodes()) != 1 {
		t.Fatal("full relative path did not match")
	}
	key(v, tcell.KeyEnter, 0)
	if v.vaultPath.String() != "parent/child" || v.IsFiltering() {
		t.Fatal("nested folder did not open")
	}
}
