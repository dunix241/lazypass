package tui

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/config"
	"lazypass/internal/vault/service"
	"testing"
)

func TestOverlaysAreExclusiveAndConsumeGlobalKeys(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: &memoryVault{}}))
	quit := false
	a.screen.quit = func() { quit = true }
	a.screen.openStoreForm()
	a.screen.openThemePanel()
	if a.screen.overlay != storeOverlay || a.screen.themePanel.IsOpen() {
		t.Fatal("theme opened over Store form")
	}
	for _, r := range "qvt" {
		themeKey(a, tcell.KeyRune, r)
	}
	if quit || a.CurrentRoute() != GeneratorRoute {
		t.Fatal("Store form leaked global input")
	}
	themeKey(a, tcell.KeyEscape, 0)
	if a.screen.overlay != noOverlay {
		t.Fatal("Store close did not release overlay")
	}
	a.screen.openThemePanel()
	a.screen.openStoreForm()
	if a.screen.overlay != themeOverlay || a.screen.store.IsOpen() {
		t.Fatal("Store opened over theme panel")
	}
	themeKey(a, tcell.KeyRune, 'q')
	if quit || a.screen.overlay != noOverlay {
		t.Fatal("theme close did not release overlay")
	}
}
