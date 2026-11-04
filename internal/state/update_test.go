package state

import (
	"lazypass/internal/update"
	"testing"
	"time"
)

func TestUpdatesAndVaultSelectionsPreserveEachOther(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := SaveVault("fixture.yaml", "personal"); err != nil {
		t.Fatal(err)
	}
	store := UpdateStore{}
	cache := update.Cache{CheckedAt: time.Now().UTC(), Dismissed: "v1.0.0"}
	if err := store.SaveUpdates(cache); err != nil {
		t.Fatal(err)
	}
	if name, err := LastVault("fixture.yaml"); err != nil || name != "personal" {
		t.Fatal("updates overwrote vault state")
	}
	if err := SaveVault("fixture.yaml", "work"); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadUpdates()
	if err != nil || got.Dismissed != cache.Dismissed || !got.CheckedAt.Equal(cache.CheckedAt) {
		t.Fatal("vault save overwrote updates")
	}
}
