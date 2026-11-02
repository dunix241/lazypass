package vault

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/vault/service"
)

func TestGeneratorEntryPrefillAndOptionalFields(t *testing.T) {
	p := &memoryVault{}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}, Password: func() string { return "generated" }})
	s.Open()
	s.storePath = "entry"
	storeKey(s, tcell.KeyEnter, 0)
	if !s.editingValue || s.value != "generated" {
		t.Fatal("generated password not editable/prefilled")
	}
	for _, value := range []string{"user", "https://example.test", "first line"} {
		storeKey(s, tcell.KeyTAB, 0)
		for _, r := range value {
			storeKey(s, tcell.KeyRune, r)
		}
	}
	storeKey(s, tcell.KeyCtrlN, 0)
	for _, r := range "second line" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyEnter, 0)
	if p.store.Password != "generated" || p.store.Username != "user" || p.store.URL != "https://example.test" || p.store.Notes != "first line\nsecond line" {
		t.Fatalf("entry fields not saved")
	}
}
