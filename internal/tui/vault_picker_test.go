package tui

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/config"
	"lazypass/internal/state"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
	"path/filepath"
	"testing"
)

func TestVaultSelectionPersistsAcrossSessions(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.Defaults()
	cfg.Vaults = config.Vaults{{Name: "first", Provider: "pass"}, {Name: "second", Provider: "age"}}
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	items := []NamedVault{{Name: "first", Service: service.Service{Provider: &memoryVault{}}}, {Name: "second", Service: service.Service{Provider: &memoryVault{}}}}
	a := NewApp(cfg, path, nil, WithVaults(items), WithInitialRoute(VaultRoute))
	a.screen.openVaultPicker()
	a.screen.handleVaultPicker(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	a.screen.handleVaultPicker(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := state.LastVault(path)
	if err != nil {
		t.Fatal(err)
	}
	if selected != "second" || loaded.LastVault != "" || len(loaded.Vaults) != 2 {
		t.Fatal("selection not saved separately from config")
	}
	loaded.LastVault = selected
	b := NewApp(loaded, "", nil, WithVaults(items))
	if b.screen.vaultIndex != 1 {
		t.Fatal("last vault not restored")
	}
	loaded.LastVault = "removed"
	c := NewApp(loaded, "", nil, WithVaults(items))
	if c.screen.vaultIndex != 0 {
		t.Fatal("missing selection did not fall back")
	}
}

type lockingVault struct {
	memoryVault
	locked bool
}

func (p *lockingVault) Lock() { p.locked = true }

func TestShutdownLocksAllVaults(t *testing.T) {
	first, second := &lockingVault{}, &lockingVault{}
	a := NewApp(config.Defaults(), "", nil, WithVaults([]NamedVault{{Name: "first", Service: service.Service{Provider: first}}, {Name: "second", Service: service.Service{Provider: second}}}))
	a.screen.closeVaults()
	if !first.locked || !second.locked {
		t.Fatal("unselected vault identity not locked")
	}
}

func TestVaultPickerPreservesFolderStateAndRoutesWrites(t *testing.T) {
	aProvider := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "folder", Path: vault.Path{"folder"}, Kind: vault.FolderNode}}}}
	bProvider := &memoryVault{}
	a := NewApp(config.Defaults(), "", nil, WithVaults([]NamedVault{{Name: "first", Service: service.Service{Provider: aProvider}}, {Name: "second", Service: service.Service{Provider: bProvider}}}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	key := func(k tcell.Key, r rune) { a.screen.InputHandler()(tcell.NewEventKey(k, r, tcell.ModNone), nil) }
	key(tcell.KeyEnter, 0)
	key(tcell.KeyRune, 'p')
	if a.screen.overlay != vaultPickerOverlay {
		t.Fatal("p did not open picker")
	}
	key(tcell.KeyDown, 0)
	key(tcell.KeyEnter, 0)
	if a.screen.vaultIndex != 1 || len(a.screen.vault.CurrentPath()) != 0 {
		t.Fatal("vault not switched")
	}
	key(tcell.KeyRune, 'a')
	for _, r := range "entry" {
		key(tcell.KeyRune, r)
	}
	key(tcell.KeyEnter, 0)
	for _, r := range "fixture" {
		key(tcell.KeyRune, r)
	}
	key(tcell.KeyEnter, 0)
	if bProvider.store.Password != "fixture" || aProvider.store.Password != "" {
		t.Fatal("write routed to wrong vault")
	}
	key(tcell.KeyRune, 'p')
	key(tcell.KeyUp, 0)
	key(tcell.KeyEnter, 0)
	if a.screen.vault.CurrentPath().String() != "folder" {
		t.Fatal("folder state lost on switch")
	}
}
