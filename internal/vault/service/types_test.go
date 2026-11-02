package service

import (
	"lazypass/internal/vault"
	"testing"
)

func TestTypeCacheDecoratesAndForgets(t *testing.T) {
	c := &TypeCache{}
	c.remember(vault.Entry{Path: vault.Path{"folder", "card"}, Type: vault.CardEntry})
	nodes := []vault.Node{{Path: vault.Path{"folder", "card"}, Kind: vault.EntryNode}}
	nodes = c.decorate(nodes)
	if !nodes[0].TypeKnown || nodes[0].EntryType != vault.CardEntry {
		t.Fatal("type not decorated")
	}
	c.forget(vault.Node{Path: vault.Path{"folder"}, Kind: vault.FolderNode})
	nodes[0].TypeKnown = false
	nodes = c.decorate(nodes)
	if nodes[0].TypeKnown {
		t.Fatal("deleted type retained")
	}
}

func TestTypeCacheDoesNotMutateProviderNodes(t *testing.T) {
	c := &TypeCache{}
	c.remember(vault.Entry{Path: vault.Path{"entry"}, Type: vault.CardEntry})
	original := []vault.Node{{Path: vault.Path{"entry"}, Kind: vault.EntryNode}}
	decorated := c.decorate(original)
	if original[0].TypeKnown || !decorated[0].TypeKnown {
		t.Fatal("provider-owned node slice mutated")
	}
	original[0].TypeKnown = true
	original[0].EntryType = vault.NoteEntry
	if got := c.decorate(original); got[0].EntryType != vault.NoteEntry {
		t.Fatal("cached type replaced native metadata")
	}
}

func TestFolderInvalidationPreservesSameNamedEntry(t *testing.T) {
	c := &TypeCache{}
	c.remember(vault.Entry{Path: vault.Path{"same"}, Type: vault.CardEntry})
	c.remember(vault.Entry{Path: vault.Path{"same", "child"}, Type: vault.NoteEntry})
	c.forget(vault.Node{Path: vault.Path{"same"}, Kind: vault.FolderNode})
	nodes := c.decorate([]vault.Node{{Path: vault.Path{"same"}, Kind: vault.EntryNode}, {Path: vault.Path{"same", "child"}, Kind: vault.EntryNode}})
	if !nodes[0].TypeKnown || nodes[1].TypeKnown {
		t.Fatal("folder deletion invalidated wrong entries")
	}
	c.clear()
	if len(c.types) != 0 {
		t.Fatal("cache not cleared")
	}
}
