package vault

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

type typedVault struct{ memoryVault }

func (p *typedVault) SupportedEntryTypes() []vault.EntryType {
	return []vault.EntryType{vault.LoginEntry, vault.CardEntry, vault.IdentityEntry, vault.NoteEntry, vault.SSHKeyEntry}
}

func TestVaultAddSelectsCardAndStoresFields(t *testing.T) {
	p := &typedVault{}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	s.OpenAdd(nil)
	if !s.choosingType {
		t.Fatal("Vault Add omitted type picker")
	}
	storeKey(s, tcell.KeyDown, 0)
	storeKey(s, tcell.KeyEnter, 0)
	for _, r := range "card" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyEnter, 0)
	for _, r := range "Fixture User" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyTAB, 0)
	for _, r := range "Visa" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyEnter, 0)
	if p.store.Type != vault.CardEntry || p.store.Fields["cardholderName"] != "Fixture User" || p.store.Fields["brand"] != "Visa" {
		t.Fatal("card fields not saved")
	}
}

func TestLoginOnlyProviderSkipsTypePicker(t *testing.T) {
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: &memoryVault{}}})
	s.OpenAdd(nil)
	if s.choosingType || s.entryType != vault.LoginEntry {
		t.Fatal("unsupported types offered")
	}
}

func TestGeneratorDoesNotExposeTypePicker(t *testing.T) {
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: &memoryVault{}}, Password: func() string { return "fixture" }})
	s.Open()
	if s.choosingType || s.entryType != vault.LoginEntry {
		t.Fatal("Generator offered non-login types")
	}
}
