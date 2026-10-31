package tui

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/config"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
	"testing"
)

func TestVaultCopyDoesNotSuspendWithoutPrompt(t *testing.T) {
	for _, mode := range []string{"suspend", "refused", "none"} {
		t.Run(mode, func(t *testing.T) {
			p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "mail", Path: vault.Path{"mail"}, Kind: vault.EntryNode}}}, read: map[string]vault.Entry{"mail": {Password: "secret"}}}
			var copied string
			calls := 0
			a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; calls++; return nil }}), WithInitialRoute(VaultRoute))
			if err := a.Init(); err != nil {
				t.Fatal(err)
			}
			if a.screen.suspend == nil {
				t.Fatal("terminal suspension not wired")
			}
			suspended := false
			switch mode {
			case "suspend":
				a.screen.suspend = func(fn func()) bool { suspended = true; fn(); return true }
			case "refused":
				a.screen.suspend = func(func()) bool { return false }
			case "none":
				a.screen.suspend = nil
			}
			a.screen.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), nil)
			if copied != "secret" || calls != 1 || a.screen.status != "Copied to clipboard" || suspended {
				t.Fatalf("copy=%q calls=%d suspended=%t", copied, calls, suspended)
			}
		})
	}
}

func TestVaultThemeShortcut(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil, WithInitialRoute(VaultRoute))
	a.screen.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone), nil)
	if !a.screen.themePanel.IsOpen() {
		t.Fatal("Vault theme shortcut not routed")
	}
}

func TestVaultFilterIsolatesGlobalShortcuts(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil, WithInitialRoute(VaultRoute))
	quit := false
	a.screen.quit = func() { quit = true }
	for _, r := range "/qvt" {
		a.screen.InputHandler()(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone), nil)
	}
	if quit || a.CurrentRoute() != VaultRoute || a.screen.themePanel.IsOpen() || !a.screen.vault.IsFiltering() {
		t.Fatal("filter leaked global shortcuts")
	}
}
