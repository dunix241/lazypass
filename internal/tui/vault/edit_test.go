package vault

import (
	"errors"
	"testing"

	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
)

func TestEditValuePreservesMetadataAndClearsState(t *testing.T) {
	p := &memoryVault{read: map[string]vault.Entry{"mail": {Path: vault.Path{"mail"}, Password: "old-secret", Username: "user", URL: "https://example.test", Notes: "notes", Fields: map[string]string{"custom": "kept"}}}}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	if !s.OpenEdit(vault.Path{"mail"}) || s.value != "old-secret" || !s.editingValue {
		t.Fatal("edit did not load value")
	}
	if s.editEntry.Password != "" {
		t.Fatal("metadata snapshot retained a redundant password")
	}
	storeKey(s, tcell.KeyCtrlU, 0)
	for _, r := range "new-secret" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyEnter, 0)
	if s.IsOpen() || s.editEntry != nil || s.value != "" {
		t.Fatal("edit state retained")
	}
	if p.store.Password != "new-secret" || p.store.Path.String() != "mail" || p.store.Username != "user" || p.store.URL != "https://example.test" || p.store.Notes != "notes" || p.store.Fields["custom"] != "kept" {
		t.Fatal("edit changed metadata")
	}
	s.OpenEdit(vault.Path{"mail"})
	storeKey(s, tcell.KeyEscape, 0)
	if s.value != "" || s.editEntry != nil {
		t.Fatal("cancel retained decrypted entry")
	}
}

func TestEditFailureAndEntryOnlyShortcut(t *testing.T) {
	p := &memoryVault{err: errors.New("secret external failure")}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	if s.OpenEdit(vault.Path{"mail"}) || s.IsOpen() {
		t.Fatal("failed read opened editor")
	}
	calls := 0
	v := New(Dependencies{OpenEdit: func(path vault.Path) {
		calls++
		if path.String() != "mail" {
			t.Fatal("wrong edit path")
		}
	}})
	v.vaultNodes = []vault.Node{{Name: "folder", Path: vault.Path{"folder"}, Kind: vault.FolderNode}, {Name: "mail", Path: vault.Path{"mail"}, Kind: vault.EntryNode}}
	key(v, tcell.KeyRune, 'e')
	if calls != 0 {
		t.Fatal("folder opened value editor")
	}
	v.vaultSelected = 1
	key(v, tcell.KeyRune, 'e')
	if calls != 1 {
		t.Fatal("e did not open entry editor")
	}
}
