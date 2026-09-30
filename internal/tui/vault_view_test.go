package tui

import (
	"fmt"
	"testing"

	"lazypass/internal/config"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestVaultBrowseAndCopy(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"":     {{Name: "team", Path: vault.Path{"team"}, Kind: vault.FolderNode}},
		"team": {{Name: "site", Path: vault.Path{"team", "site"}, Kind: vault.EntryNode}},
	}, read: map[string]vault.Entry{"team/site": {Password: "secret"}}}
	var copied string
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.openVaultNode()
	if got := a.screen.vaultPath.String(); got != "team" {
		t.Fatalf("path = %q", got)
	}
	a.screen.openVaultNode()
	if copied != "secret" || a.screen.status != "Copied to clipboard" || a.screen.statusLevel != notificationSuccess {
		t.Fatalf("copy = %q, notification = %q/%d", copied, a.screen.status, a.screen.statusLevel)
	}
}

func TestVaultCopyReleasesTerminalForPrompts(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"": {{Name: "mail", Path: vault.Path{"mail"}, Kind: vault.EntryNode}},
	}, read: map[string]vault.Entry{"mail": {Password: "secret"}}}
	var copied string
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if a.screen.suspend == nil {
		t.Fatal("screen should wire terminal suspension by default")
	}
	var suspended bool
	a.screen.suspend = func(fn func()) bool { suspended = true; fn(); return true }
	a.screen.copyVaultEntry()
	if !suspended {
		t.Fatal("vault copy should release the terminal for pinentry")
	}
	if copied != "secret" || a.screen.status != "Copied to clipboard" {
		t.Fatalf("copy=%q status=%q", copied, a.screen.status)
	}
}

func TestVaultCopyFallsBackWithoutSuspend(t *testing.T) {
	vaultApp := func() (*App, *string) {
		p := &memoryVault{nodes: map[string][]vault.Node{
			"": {{Name: "mail", Path: vault.Path{"mail"}, Kind: vault.EntryNode}},
		}, read: map[string]vault.Entry{"mail": {Password: "secret"}}}
		var copied string
		a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}), WithInitialRoute(VaultRoute))
		if err := a.Init(); err != nil {
			t.Fatal(err)
		}
		return a, &copied
	}
	a, copied := vaultApp()
	a.screen.suspend = func(fn func()) bool { fn(); return false }
	a.screen.copyVaultEntry()
	if *copied != "secret" {
		t.Fatal("copy should run directly when suspension is refused")
	}
	a, copied = vaultApp()
	a.screen.suspend = nil
	a.screen.copyVaultEntry()
	if *copied != "secret" {
		t.Fatal("copy should run directly without a suspend hook")
	}
}

func TestCopyConfirmationDoesNotBlockVaultNavigation(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"": {{Name: "first", Path: vault.Path{"first"}, Kind: vault.EntryNode}, {Name: "second", Path: vault.Path{"second"}, Kind: vault.EntryNode}},
	}, read: map[string]vault.Entry{"first": {Password: "one"}, "second": {Password: "two"}}}
	var copied string
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.copyVaultEntry()
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	a.screen.copyVaultEntry()
	if copied != "two" || a.screen.vaultSelected != 1 {
		t.Fatalf("copy=%q selected=%d", copied, a.screen.vaultSelected)
	}
}

func TestVaultVimNavigation(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"":     {{Name: "alpha", Path: vault.Path{"alpha"}, Kind: vault.FolderNode}, {Name: "team", Path: vault.Path{"team"}, Kind: vault.FolderNode}},
		"team": {{Name: "site", Path: vault.Path{"team", "site"}, Kind: vault.EntryNode}},
	}, read: map[string]vault.Entry{"team/site": {Password: "secret"}}}
	var copied string
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.vaultSelected = 1
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone))
	if got := a.screen.vaultPath.String(); got != "team" {
		t.Fatalf("l should open folder, path = %q", got)
	}
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone))
	if copied != "secret" {
		t.Fatal("l should copy the selected entry")
	}
	copied = ""
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))
	if copied != "" {
		t.Fatal("c should not copy entries in Vault")
	}
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if copied != "secret" {
		t.Fatal("Enter should copy the selected entry")
	}
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone))
	if len(a.screen.vaultPath) != 0 || a.screen.vaultSelected != 1 {
		t.Fatalf("h should restore the parent selection, path = %q selected = %d", a.screen.vaultPath, a.screen.vaultSelected)
	}
}

func TestVaultMouseMatchesScrolledRows(t *testing.T) {
	nodes := make([]vault.Node, 20)
	for i := range nodes {
		name := fmt.Sprintf("entry-%02d", i)
		nodes[i] = vault.Node{Name: name, Path: vault.Path{name}, Kind: vault.EntryNode}
	}
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: &memoryVault{nodes: map[string][]vault.Node{"": nodes}}}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.SetRect(0, 0, 50, 18)
	l := calculateLayout(50, 18)
	a.screen.vaultSelected = 18
	listY, _, start := a.screen.vaultListWindow(l.card)
	event := tcell.NewEventMouse(l.card.x+2, listY, tcell.Button1, tcell.ModNone)
	a.screen.handleVaultMouse(tview.MouseLeftClick, event, l, 0, 0)
	if a.screen.vaultSelected != start {
		t.Fatalf("clicked index = %d, visible first = %d", a.screen.vaultSelected, start)
	}
	a.screen.vaultFiltering, a.screen.vaultFilter, a.screen.vaultSelected = true, "entry", 18
	listY, _, start = a.screen.vaultListWindow(l.card)
	event = tcell.NewEventMouse(l.card.x+2, listY, tcell.Button1, tcell.ModNone)
	a.screen.handleVaultMouse(tview.MouseLeftClick, event, l, 0, 0)
	if a.screen.vaultSelected != start {
		t.Fatalf("filtered clicked index = %d, visible first = %d", a.screen.vaultSelected, start)
	}
}

func TestVaultFilterCombinesTypingAndPickerNavigation(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"": {{Name: "github", Path: vault.Path{"github"}, Kind: vault.EntryNode}, {Name: "gitlab", Path: vault.Path{"gitlab"}, Kind: vault.EntryNode}, {Name: "personal", Path: vault.Path{"personal"}, Kind: vault.FolderNode}},
	}, read: map[string]vault.Entry{"github": {Password: "one"}, "gitlab": {Password: "two"}}}
	var copied string
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	a.screen.handleVaultFilterInput(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))
	if !a.screen.vaultFiltering || len(a.screen.filteredVaultNodes()) != 2 {
		t.Fatalf("filter=%q nodes=%#v", a.screen.vaultFilter, a.screen.filteredVaultNodes())
	}
	a.screen.handleVaultFilterInput(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	a.screen.handleVaultFilterInput(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if copied != "two" {
		t.Fatalf("picker copied %q", copied)
	}
	a.screen.handleVaultFilterInput(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if a.screen.vaultFiltering || a.screen.vaultFilter != "" {
		t.Fatal("escape should close and clear the filter")
	}
}

func TestVaultNodeLabelsAndColors(t *testing.T) {
	folder := vault.Node{Name: "personal", Kind: vault.FolderNode}
	entry := vault.Node{Name: "proton", Kind: vault.EntryNode}
	a := NewApp(config.Defaults(), "", nil)
	if got := a.screen.vaultNodeLabel(folder); got != " personal/" {
		t.Fatalf("default folder label = %q", got)
	}
	if got := a.screen.vaultNodeLabel(entry); got != "󰦝 proton" {
		t.Fatalf("default entry label = %q", got)
	}
	if a.screen.vaultNodeColor(folder) != a.screen.colors.accent || a.screen.vaultNodeColor(entry) != a.screen.colors.text {
		t.Fatal("vault node colors should distinguish folders from entries")
	}
	cfg := config.Defaults()
	cfg.NerdFont = false
	a = NewApp(cfg, "", nil)
	if got := a.screen.vaultNodeLabel(folder); got != "personal/" {
		t.Fatalf("ASCII folder label = %q", got)
	}
	if got := a.screen.vaultNodeLabel(entry); got != "proton" {
		t.Fatalf("ASCII entry label = %q", got)
	}
}

func TestVaultMouseSelectsAndActivates(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"":     {{Name: "alpha", Path: vault.Path{"alpha"}, Kind: vault.FolderNode}, {Name: "team", Path: vault.Path{"team"}, Kind: vault.FolderNode}},
		"team": {{Name: "site", Path: vault.Path{"team", "site"}, Kind: vault.EntryNode}},
	}, read: map[string]vault.Entry{"team/site": {Password: "secret"}}}
	var copied string
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.SetRect(0, 0, 80, 24)
	l := calculateLayout(80, 24)
	r := l.card
	listY := r.y + 2
	at := func(row int) *tcell.EventMouse {
		return tcell.NewEventMouse(r.x+2, listY+row, tcell.Button1, tcell.ModNone)
	}
	if a.screen.vaultSelected != 0 {
		t.Fatalf("initial selected = %d", a.screen.vaultSelected)
	}
	a.screen.handleVaultMouse(tview.MouseLeftClick, at(1), l, 0, 0)
	if a.screen.vaultSelected != 1 {
		t.Fatalf("click should select row, selected = %d", a.screen.vaultSelected)
	}
	if got := a.screen.vaultPath.String(); got != "" {
		t.Fatalf("first click should not open folder, path = %q", got)
	}
	a.screen.handleVaultMouse(tview.MouseLeftClick, at(1), l, 0, 0)
	if got := a.screen.vaultPath.String(); got != "team" {
		t.Fatalf("second click should open folder, path = %q", got)
	}
	a.screen.handleVaultMouse(tview.MouseLeftDoubleClick, at(1), l, 0, 0)
	if copied != "secret" {
		t.Fatalf("double-click should copy entry, copied = %q", copied)
	}
	a.screen.handleVaultMouse(tview.MouseScrollDown, at(0), l, 0, 0)
	a.screen.handleVaultMouse(tview.MouseScrollUp, at(0), l, 0, 0)
	a.screen.handleVaultMouse(tview.MouseLeftClick, at(0), l, 0, 0)
	a.screen.handleVaultMouse(tview.MouseLeftClick, at(0), l, 0, 0)
	if got := a.screen.vaultPath.String(); got != "" {
		t.Fatalf("../ clicks should go to parent, path = %q", got)
	}
	a.screen.handleVaultMouse(tview.MouseLeftClick, at(1), l, 0, 0)
	a.screen.handleVaultMouse(tview.MouseLeftClick, at(1), l, 0, 0)
	a.screen.handleVaultMouse(tview.MouseRightClick, at(0), l, 0, 0)
	if got := a.screen.vaultPath.String(); got != "" {
		t.Fatalf("right-click should go to parent, path = %q", got)
	}
}

func TestVaultThemeShortcut(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil, WithInitialRoute(VaultRoute))
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
	if !a.screen.themePanel.open {
		t.Fatal("t should open the theme picker in vault view")
	}
}
