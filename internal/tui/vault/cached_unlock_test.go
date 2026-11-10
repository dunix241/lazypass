package vault

import (
	"testing"

	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

func TestCachedBrowsingDoesNotWaitForUnlockOrReadProvider(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"folder": {{Name: "fresh", Path: vault.Path{"folder", "fresh"}, Kind: vault.EntryNode}},
	}}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.cachedLocations = map[string][]vault.Node{
		"":       {{Name: "folder", Path: vault.Path{"folder"}, Kind: vault.FolderNode}},
		"folder": {{Name: "cached", Path: vault.Path{"folder", "cached"}, Kind: vault.EntryNode}},
	}
	v.BrowseCached()
	if !v.awaitingUnlock || len(v.vaultNodes) != 1 || v.vaultNodes[0].Name != "folder" {
		t.Fatal("cached rows unavailable while unlocking")
	}
	v.navigate(vault.Path{"folder"}, 1, "")
	if len(v.vaultNodes) != 1 || v.vaultNodes[0].Name != "cached" {
		t.Fatal("navigation fetched provider before authentication")
	}
	v.Authenticated()
	if v.awaitingUnlock || len(v.vaultNodes) != 1 || v.vaultNodes[0].Name != "fresh" {
		t.Fatal("unlock did not refresh current cached location")
	}
}
