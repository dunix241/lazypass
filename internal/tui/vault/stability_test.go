package vault

import (
	"context"
	"testing"

	"lazypass/internal/vault"
)

func TestFlatCollectionReferenceReturnsToVisualParent(t *testing.T) {
	v := New(Dependencies{RunOperation: func(func(context.Context) error, func(error)) {}})
	parent := vault.Path{"Organizations", "org", "group-parent"}
	child := vault.Path{"Organizations", "org", "collection-id"}
	v.rememberLocationParents(parent, []vault.Node{{Path: child, Name: "child", Kind: vault.FolderNode}})
	v.vaultPath = child
	if v.vaultParentNode().Path.String() != parent.String() {
		t.Fatal("parent row uses structural rather than visual parent")
	}
	v.vaultParent()
	if v.vaultPath.String() != parent.String() {
		t.Fatal("back skipped visual parent")
	}
}
