package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"lazypass/internal/config"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
)

func TestCopyStatusReflectsClipboardOutcome(t *testing.T) {
	success := NewApp(config.Defaults(), "", func(string) error { return nil })
	success.screen.password = "password"
	success.screen.copy()
	if success.screen.status != "Copied to clipboard" || success.screen.statusLevel != notificationSuccess {
		t.Fatalf("success notification = %q/%d", success.screen.status, success.screen.statusLevel)
	}

	failure := NewApp(config.Defaults(), "", func(string) error { return errors.New("no clipboard") })
	failure.screen.password = "password"
	failure.screen.copy()
	if failure.screen.status != "Clipboard unavailable" || failure.screen.statusLevel != notificationError {
		t.Fatalf("failure notification = %q/%d", failure.screen.status, failure.screen.statusLevel)
	}
}

func TestCopyErrorMessageMapsFailureCauses(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{vault.ErrLocked, "Vault locked (no details)"},
		{safeDiagnosticError{}, "GPG: no terminal for pinentry"},
		{vault.ErrNotFound, "Vault entry not found"},
		{vault.ErrMalformed, "Vault entry format invalid"},
		{vault.ErrUninitialized, "Vault not initialized"},
		{vault.ErrUnavailable, "Vault unavailable"},
		{service.ErrClipboard, "Clipboard unavailable"},
		{errors.New("write failed"), "Copy failed"},
	} {
		if got := copyErrorMessage(test.err); got != test.want {
			t.Fatalf("copyErrorMessage(%v) = %q, want %q", test.err, got, test.want)
		}
	}
	if got := storeErrorMessage(safeDiagnosticError{}); got != "GPG: no terminal for pinentry" {
		t.Fatalf("store diagnostic = %q", got)
	}
}

func TestVaultCopyFailureLogsOnlyClassifiedCause(t *testing.T) {
	state := t.TempDir()
	t.Setenv("LAZYPASS_DEBUG", "1")
	t.Setenv("XDG_STATE_HOME", state)
	p := &memoryVault{nodes: map[string][]vault.Node{
		"": {{Name: "mail", Path: vault.Path{"mail"}, Kind: vault.EntryNode}},
	}, read: map[string]vault.Entry{"mail": {Password: "secret-value"}}}
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(string) error { return errors.New("secret-value in failure") }}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.copyVaultEntry()
	if a.screen.status != "Clipboard unavailable" {
		t.Fatalf("notification = %q", a.screen.status)
	}
	log, err := os.ReadFile(filepath.Join(state, "lazypass", "debug.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), `cause="clipboard"`) || strings.Contains(string(log), "secret-value") {
		t.Fatalf("unsafe debug log: %q", log)
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
	if a.screen.notificationColor(notificationInfo) != a.screen.colors.focus ||
		a.screen.notificationColor(notificationSuccess) != a.screen.colors.accent ||
		a.screen.notificationColor(notificationWarning) != a.screen.colors.warning ||
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

func TestGeneratorFooterShowsOnlyFocusedControlHint(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	for _, width := range []int{120, 84, 50, 32} {
		for _, test := range []struct {
			focus focus
			want  string
			omit  string
		}{
			{focusLength, "<left>/<right> length", "<space> toggle"},
			{focusSymbols, "<space> toggle", "<left>/<right> length"},
			{focusStore, "", "<space> toggle"},
		} {
			a.screen.selected = test.focus
			footer := a.screen.generatorFooter(width)
			if utf8.RuneCountInString(footer) > width-2 || !strings.Contains(footer, test.want) || strings.Contains(footer, test.omit) || strings.Contains(footer, "U/L/N/S/E") || strings.Contains(footer, "a Add") {
				t.Fatalf("width=%d focus=%d footer=%q", width, test.focus, footer)
			}
			if width >= 84 {
				copyAt, themesAt, viewAt := strings.Index(footer, "c copy"), strings.Index(footer, "t themes"), strings.Index(footer, "v view")
				if copyAt >= themesAt || themesAt >= viewAt {
					t.Fatalf("theme hint should follow copy and precede view: %q", footer)
				}
			}
		}
	}
}
