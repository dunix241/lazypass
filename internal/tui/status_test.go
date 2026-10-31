package tui

import (
	"errors"
	"strings"
	"testing"

	"lazypass/internal/config"

	"github.com/gdamore/tcell/v2"
)

func TestCopyStatusReflectsClipboardOutcome(t *testing.T) {
	success := NewApp(config.Defaults(), "", func(string) error { return nil })
	success.screen.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone), nil)
	if success.screen.status != "Copied to clipboard" || success.screen.statusLevel != notificationSuccess {
		t.Fatalf("success notification = %q/%d", success.screen.status, success.screen.statusLevel)
	}

	failure := NewApp(config.Defaults(), "", func(string) error { return errors.New("no clipboard") })
	failure.screen.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone), nil)
	if failure.screen.status != "Clipboard unavailable" || failure.screen.statusLevel != notificationError {
		t.Fatalf("failure notification = %q/%d", failure.screen.status, failure.screen.statusLevel)
	}
}

func TestNarrowStatusRemainsVisible(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	a.screen.notify(notificationError, "Clipboard unavailable")
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(50, 18)
	a.screen.SetRect(0, 0, 50, 18)
	a.screen.Draw(sim)
	var row strings.Builder
	for x := 0; x < 50; x++ {
		cell, _, _ := sim.Get(x, 0)
		row.WriteString(cell)
	}
	if !strings.Contains(row.String(), "Clipboard") {
		t.Fatalf("notification hidden behind logo: %q", row.String())
	}
}

func TestNotificationLevelsUseDistinctColors(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	if a.screen.notificationColor(notificationInfo) != a.screen.colors.Focus ||
		a.screen.notificationColor(notificationSuccess) != a.screen.colors.Accent ||
		a.screen.notificationColor(notificationWarning) != a.screen.colors.Warning ||
		a.screen.notificationColor(notificationError) != tcell.ColorRed {
		t.Fatal("notification levels should use their semantic colors")
	}
}

func TestVaultFooterUsesVaultHintsAtEveryWidth(t *testing.T) {
	for _, width := range []int{84, 50, 32} {
		a := NewApp(config.Defaults(), "", nil, WithInitialRoute(VaultRoute))
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(width, 24)
		a.screen.SetRect(0, 0, width, 24)
		a.screen.Draw(sim)
		var footer strings.Builder
		for x := 0; x < width; x++ {
			cell, _, _ := sim.Get(x, 23)
			footer.WriteString(cell)
		}
		sim.Fini()
		text := footer.String()
		if !strings.Contains(text, "h back") || !strings.Contains(text, "l open/copy") || !strings.Contains(text, "q quit") || strings.Contains(text, "a Add") || strings.Contains(text, "c copy") {
			t.Fatalf("%d-column Vault footer = %q", width, text)
		}
	}
}
