package vault

import (
	"context"
	"errors"
	"testing"

	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

func TestCachedRowsRemainInteractiveDuringRefresh(t *testing.T) {
	for _, empty := range []bool{false, true} {
		var finish func(error)
		v := New(Dependencies{RunOperation: func(_ func(context.Context) error, done func(error)) { finish = done }})
		nodes := []vault.Node{{Path: vault.Path{"cached"}, Name: "cached", Kind: vault.EntryNode}}
		if empty {
			nodes = nil
		}
		v.cachedLocations = map[string][]vault.Node{"": nodes}
		v.Refresh()
		if v.loading || !v.refreshing {
			t.Fatal("cached location hidden behind loading")
		}
		if len(v.vaultNodes) != len(nodes) {
			t.Fatal("cached rows missing")
		}
		// A failed refresh must leave the previous snapshot intact.
		finish(errors.New("offline"))
		if v.loading || v.refreshing || len(v.vaultNodes) != len(nodes) {
			t.Fatal("failed refresh discarded cached snapshot")
		}
	}
}

func TestRefreshPreservesMovedSelectionAfterRowsReorder(t *testing.T) {
	a := vault.Node{Path: vault.Path{"a"}, Name: "a", Kind: vault.FolderNode}
	b := vault.Node{Path: vault.Path{"b"}, Name: "b", Kind: vault.FolderNode}
	provider := &memoryVault{nodes: map[string][]vault.Node{"": {b, a}}}
	var finish func()
	v := New(Dependencies{Service: service.Service{Provider: provider}, RunOperation: func(work func(context.Context) error, done func(error)) {
		finish = func() { done(work(context.Background())) }
	}})
	v.cachedLocations = map[string][]vault.Node{"": {a, b}}
	v.refresh("a")
	v.moveVaultSelection(1)
	finish()
	node, ok := v.selectedVaultNode()
	if !ok || node.Path.String() != "b" {
		t.Fatalf("refresh lost user selection: %v", node)
	}
}

func TestNavigationReplacesParentRowsBeforeRefreshCompletes(t *testing.T) {
	var completions []func(error)
	v := New(Dependencies{RunOperation: func(_ func(context.Context) error, done func(error)) {
		completions = append(completions, done)
	}})
	parent := vault.Path{"Organizations", "org"}
	child := vault.Path{"Organizations", "org", "collection"}
	v.vaultPath = parent
	v.vaultNodes = []vault.Node{{Path: child, Name: "collection", Kind: vault.FolderNode}}
	v.vaultSelected = 1 // Parent navigation row precedes the collection.
	v.cachedLocations = map[string][]vault.Node{
		child.String(): {{Path: append(append(vault.Path(nil), child...), "item"), Name: "cached child entry", Kind: vault.EntryNode}},
	}
	v.Refresh()
	v.openVaultNode()
	if v.vaultPath.String() != child.String() || len(v.vaultNodes) != 1 || v.vaultNodes[0].Name != "cached child entry" {
		t.Fatalf("navigation retained parent rows: path=%s nodes=%v", v.vaultPath, v.vaultNodes)
	}
	if v.loading {
		t.Fatal("cached destination blocked navigation")
	}
	completions[0](nil)
	if len(v.vaultNodes) != 1 || v.vaultNodes[0].Name != "cached child entry" {
		t.Fatal("stale parent refresh replaced child rows")
	}
}

func TestUncachedNavigationNeverDisplaysParentRows(t *testing.T) {
	v := New(Dependencies{RunOperation: func(func(context.Context) error, func(error)) {}})
	v.vaultNodes = []vault.Node{{Path: vault.Path{"child"}, Name: "child", Kind: vault.FolderNode}}
	v.openVaultNode()
	if len(v.vaultNodes) != 0 || !v.loading {
		t.Fatal("uncached destination displayed parent rows")
	}
}

func TestParentNavigationSelectsCachedChildImmediately(t *testing.T) {
	var complete func(error)
	v := New(Dependencies{RunOperation: func(_ func(context.Context) error, done func(error)) { complete = done }})
	parent := vault.Path{"Personal"}
	child := vault.Path{"Personal", "work"}
	v.vaultPath = child
	v.cachedLocations = map[string][]vault.Node{
		parent.String(): {
			{Path: vault.Path{"Personal", "other"}, Name: "other", Kind: vault.FolderNode},
			{Path: child, Name: "work", Kind: vault.FolderNode},
		},
	}
	v.vaultParent()
	node, ok := v.selectedVaultNode()
	if !ok || node.Path.String() != child.String() {
		t.Fatalf("cached parent selected wrong row: %v", node)
	}
	// Keep this test's provider result empty; completion should not panic when a
	// selected cached row disappears from the new snapshot.
	complete(nil)
}
