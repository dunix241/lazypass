package vault

import (
	"testing"

	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
)

func TestAddRejectsOnlyExistingEntry(t *testing.T) {
	for _, kind := range []vault.NodeKind{vault.EntryNode, vault.FolderNode} {
		p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "hello", Path: vault.Path{"hello"}, Kind: kind}}}}
		s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}, Password: func() string { return "generated" }})
		s.OpenAdd(nil)
		for _, r := range "hello" {
			storeKey(s, tcell.KeyRune, r)
		}
		storeKey(s, tcell.KeyEnter, 0)
		if s.editingValue != (kind == vault.FolderNode) || !s.IsOpen() || p.store.Password != "" {
			t.Fatal("Add should block existing values, not folders")
		}
		s.Open()
		s.storePath = "hello"
		storeKey(s, tcell.KeyEnter, 0)
		if kind == vault.EntryNode && (!s.IsOpen() || p.store.Password != "") {
			t.Fatal("Generator Add overwrote value")
		}
		if kind == vault.FolderNode && (s.IsOpen() || p.store.Password != "generated") {
			t.Fatal("Generator Add rejected a folder with the same name")
		}
	}
}

func TestAddRechecksDestinationAtSave(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{}}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	s.OpenAdd(nil)
	for _, r := range "hello" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyEnter, 0)
	p.nodes[""] = []vault.Node{{Name: "hello", Path: vault.Path{"hello"}, Kind: vault.EntryNode}}
	for _, r := range "new-value" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyEnter, 0)
	if !s.IsOpen() || p.store.Password != "" {
		t.Fatal("destination created during input was overwritten")
	}
}

func TestAddAllowsExistingParentName(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "hello", Path: vault.Path{"hello"}, Kind: vault.EntryNode}}}}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	s.OpenAdd(nil)
	for _, r := range "hello/world" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyEnter, 0)
	if !s.editingValue {
		t.Fatal("existing parent name blocked nested value")
	}
}
