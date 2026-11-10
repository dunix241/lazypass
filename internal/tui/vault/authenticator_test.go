package vault

import (
	"context"
	"testing"

	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
)

func TestAuthenticatorEditorMasksAndClearsExplicitly(t *testing.T) {
	p := &memoryVault{read: map[string]vault.Entry{
		"entry": {Path: vault.Path{"entry"}, Password: "password", Authenticator: "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP"},
	}}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	s.OpenEdit(vault.Path{"entry"})
	fields := s.entryFields()
	index := len(fields) - 1
	if fields[index].key != "authenticator" || !fields[index].secret {
		t.Fatal("authenticator input is not masked")
	}
	s.selectField(index)
	s.authenticator = ""
	s.store()
	entry := p.store
	if entry.Authenticator != "" || !entry.AuthenticatorChanged {
		t.Fatal("clearing authenticator did not produce an explicit edit")
	}
}

func TestCanceledAuthenticatorCopyDoesNotTouchClipboard(t *testing.T) {
	p := &memoryVault{
		nodes: map[string][]vault.Node{"": {{Path: vault.Path{"entry"}, Name: "entry", Kind: vault.EntryNode}}},
		read:  map[string]vault.Entry{"entry": {Authenticator: "JBSWY3DPEHPK3PXP"}},
	}
	var work func(context.Context) error
	var done func(error)
	copied := false
	v := New(Dependencies{
		Service:      service.Service{Provider: p, Copy: func(string) error { copied = true; return nil }},
		RunOperation: func(w func(context.Context) error, d func(error)) { work, done = w, d },
	})
	v.vaultNodes = p.nodes[""]
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'o', 0))
	v.CancelCopy()
	err := work(context.Background())
	done(err)
	if err != context.Canceled || copied || v.copying {
		t.Fatal("stale copy was not canceled")
	}
}

func TestLoginSubmitAllowsEmptyPasswordAndUsername(t *testing.T) {
	const uri = "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP"
	for _, editing := range []bool{false, true} {
		p := &memoryVault{read: map[string]vault.Entry{
			"entry": {Path: vault.Path{"entry"}, Authenticator: uri},
		}}
		s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
		if editing {
			s.OpenEdit(vault.Path{"entry"})
		} else {
			s.OpenAdd(nil)
			s.choosingType = false
			s.storePath = "entry"
			s.editingValue = true
			s.authenticator = uri
		}
		s.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
		if s.IsOpen() || p.store.Password != "" || p.store.Username != "" || p.store.Authenticator != uri {
			t.Fatal("authenticator-only Login was not saved")
		}
	}
}

func TestEmptyPasswordLoginPreservesUnavailableAuthenticator(t *testing.T) {
	p := &memoryVault{read: map[string]vault.Entry{
		"entry": {Path: vault.Path{"entry"}, AuthenticatorUnavailable: true},
	}}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	s.OpenEdit(vault.Path{"entry"})
	s.notes = "updated notes"
	s.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if s.IsOpen() || p.store.AuthenticatorChanged || !p.store.AuthenticatorUnavailable || p.store.Notes != "updated notes" {
		t.Fatal("editing empty-password Login changed unavailable authenticator")
	}
}
