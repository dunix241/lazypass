package tui

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/config"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
	"testing"
)

func TestStoreActionOpensStoreForm(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{}}
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p}))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	key := func(k tcell.Key, r rune) { a.screen.InputHandler()(tcell.NewEventKey(k, r, tcell.ModNone), nil) }
	key(tcell.KeyRune, 'a')
	if !a.screen.store.IsOpen() {
		t.Fatal("a did not open Store form")
	}
	for _, r := range "folder/name" {
		key(tcell.KeyRune, r)
	}
	key(tcell.KeyEnter, 0)
	key(tcell.KeyEnter, 0)
	if a.screen.store.IsOpen() || p.store.Path.String() != "folder/name" || p.store.Password != a.CurrentPassword() {
		t.Fatal("cross-view password/destination wiring failed")
	}
	key(tcell.KeyRune, 'a')
	key(tcell.KeyEscape, 0)
	before := a.CurrentOptions().Symbols
	key(tcell.KeyRune, 's')
	if a.CurrentOptions().Symbols == before {
		t.Fatal("Symbols shortcut did not resume")
	}
}

func TestVaultAddUsesManualValueAndRefreshes(t *testing.T) {
	p := &memoryVault{}
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	key := func(k tcell.Key, r rune) { a.screen.InputHandler()(tcell.NewEventKey(k, r, tcell.ModNone), nil) }
	key(tcell.KeyRune, 'a')
	if a.screen.overlay != storeOverlay {
		t.Fatal("Vault a did not open Add")
	}
	for _, r := range "folder/value" {
		key(tcell.KeyRune, r)
	}
	key(tcell.KeyEnter, 0)
	for _, r := range "already-have-this" {
		key(tcell.KeyRune, r)
	}
	key(tcell.KeyEnter, 0)
	if a.screen.overlay != noOverlay || p.store.Path.String() != "folder/value" || p.store.Password != "already-have-this" {
		t.Fatal("Vault manual Add not wired")
	}
}
