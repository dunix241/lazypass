package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

import "lazypass/internal/config"

func TestRegenerateProducesPassword(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	if err := a.screen.regenerate(); err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if len(a.CurrentPassword()) != 20 {
		t.Fatalf("got len %d want 20", len(a.CurrentPassword()))
	}
}

func TestToggleKeepsOneClass(t *testing.T) {
	c := config.Defaults()
	c.Upper, c.Lower, c.Numbers, c.Symbols = true, false, false, false
	a := NewApp(c, "", nil)
	a.screen.selected = focusUpper
	before := a.CurrentPassword()
	a.screen.activate() // would disable the last class → must revert
	if !a.CurrentOptions().Upper {
		t.Fatal("should keep last class enabled")
	}
	if a.CurrentPassword() != before {
		t.Fatal("rejecting the toggle must not regenerate the password")
	}
}

func TestLengthTypingSupportsMoreThanFifty(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	a.screen.lengthText = ""
	a.screen.typeLength('1')
	a.screen.typeLength('2')
	a.screen.typeLength('8')
	if got := a.CurrentOptions().Length; got != 128 {
		t.Fatalf("got length %d, want 128", got)
	}
}

func TestInitialRenderDoesNotCreateConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewApp(config.Defaults(), path, nil)
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("initial render should not save config, got %v", err)
	}
}

func TestTUIOptionChangesPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewApp(config.Defaults(), path, nil)
	a.screen.setLength(128)
	a.screen.selected = focusSymbols
	a.screen.activate()
	a.screen.selected = focusAmbiguous
	a.screen.activate()

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Length != 128 || !loaded.Symbols || !loaded.ExcludeAmbiguous {
		t.Fatalf("saved options were not restored: %+v", loaded)
	}
}

func TestCopyStatusReflectsClipboardOutcome(t *testing.T) {
	success := NewApp(config.Defaults(), "", func(string) error { return nil })
	success.screen.password = "password"
	success.screen.copy()
	if success.screen.status != "Copied to clipboard" {
		t.Fatalf("success status = %q", success.screen.status)
	}

	failure := NewApp(config.Defaults(), "", func(string) error { return errors.New("no clipboard") })
	failure.screen.password = "password"
	failure.screen.copy()
	if failure.screen.status != "Copy failed" {
		t.Fatalf("failure status = %q", failure.screen.status)
	}
}

func TestGlobalOptionShortcuts(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	a.screen.selected = focusSymbols
	handler := a.screen.InputHandler()
	before := a.CurrentOptions().Length
	handler(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone), nil)
	if a.screen.selected != focusLength {
		t.Fatal("Right should adjust and focus Length")
	}
	if got := a.CurrentOptions().Length; got != before+1 {
		t.Fatalf("Right increased length to %d, want %d", got, before+1)
	}
	handler(tcell.NewEventKey(tcell.KeyRune, 'U', tcell.ModShift), nil)
	if a.CurrentOptions().Upper {
		t.Fatal("U should toggle Uppercase")
	}
	handler(tcell.NewEventKey(tcell.KeyRune, 'L', tcell.ModShift), nil)
	if a.CurrentOptions().Lower {
		t.Fatal("Shift+L should toggle Lowercase")
	}
	handler(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone), nil)
	if !a.CurrentOptions().Lower {
		t.Fatal("l should toggle Lowercase")
	}
}

func TestLengthFocusIsRenderedAfterGlobalShortcut(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	a.screen.selected = focusSymbols
	handler := a.screen.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone), nil)
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(84, 24)
	a.screen.SetRect(0, 0, 84, 24)
	a.screen.Draw(sim)

	l := calculateLayout(84, 24)
	cell, style, _ := sim.Get(l.card.x+2, l.card.y+6)
	foreground, _, _ := style.Decompose()
	if cell != "L" || foreground != a.screen.colors.focus {
		t.Fatalf("length label = %q with color %v, want focused Length", cell, foreground)
	}
}

func TestSliderPointerClampsAndSetsLength(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	l := calculateLayout(84, 24)
	railStart, railWidth := sliderRail(l.length)
	a.screen.setLengthFromPointer(l.length, 0, 0, railStart+railWidth-1)
	if got := a.CurrentOptions().Length; got != 256 {
		t.Fatalf("right rail endpoint got %d, want 256", got)
	}
	a.screen.setLengthFromPointer(l.length, 0, 0, railStart-20)
	if got := a.CurrentOptions().Length; got != 4 {
		t.Fatalf("left of rail got %d, want 4", got)
	}
}

func TestDrawPaintsEntireViewport(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	if err := a.screen.regenerate(); err != nil {
		t.Fatal(err)
	}
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(84, 24)
	a.screen.SetRect(0, 0, 84, 24)
	a.screen.Draw(sim)

	_, style, _ := sim.Get(83, 23)
	foreground, background, _ := style.Decompose()
	if background != a.screen.colors.base {
		t.Fatalf("viewport background = %v, want %v (foreground %v)", background, a.screen.colors.base, foreground)
	}
	logo := calculateLogoLayout(84, 24)
	if logo.style != logoWordmark {
		t.Fatalf("84x24 logo style = %d, want wordmark", logo.style)
	}
	if logo.x != (84-logo.w)/2 {
		t.Fatalf("logo x = %d, want centered %d", logo.x, (84-logo.w)/2)
	}
	wordmark, _, _ := sim.Get(logo.x, 0)
	if wordmark != "l" {
		t.Fatalf("logo wordmark was not rendered, got %q", wordmark)
	}
}

func TestDrawRendersLiteralShortcutBadges(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	if err := a.screen.regenerate(); err != nil {
		t.Fatal(err)
	}
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(84, 24)
	a.screen.SetRect(0, 0, 84, 24)
	a.screen.Draw(sim)

	l := calculateLayout(84, 24)
	badgeX := l.upper.x + 18
	cell, _, _ := sim.Get(badgeX, l.upper.y)
	if cell != "[" {
		t.Fatalf("shortcut badge starts with %q, want '['", cell)
	}
}

func TestCompactAmbigHidesExample(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	if err := a.screen.regenerate(); err != nil {
		t.Fatal(err)
	}
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(50, 18)
	a.screen.SetRect(0, 0, 50, 18)
	a.screen.Draw(sim)

	l := calculateLayout(50, 18)
	if !l.compact {
		t.Fatalf("50x18 should use compact layout: %+v", l)
	}
	var row strings.Builder
	for xx := l.ambig.x; xx < l.ambig.x+l.ambig.w; xx++ {
		cell, _, _ := sim.Get(xx, l.ambig.y)
		row.WriteString(cell)
	}
	if !strings.HasPrefix(strings.TrimSpace(row.String()), "[ ] Exclude ambigu") {
		t.Fatalf("compact ambig row missing full label, got %q", row.String())
	}
	if strings.Contains(row.String(), "(I") {
		t.Fatalf("compact ambig row should hide the example, got %q", row.String())
	}
}

func TestToggleDoesNotShiftBadge(t *testing.T) {
	ambigRow := func(exclude bool) string {
		c := config.Defaults()
		c.ExcludeAmbiguous = exclude
		a := NewApp(c, "", nil)
		if err := a.screen.regenerate(); err != nil {
			t.Fatal(err)
		}
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		defer sim.Fini()
		sim.SetSize(84, 24)
		a.screen.SetRect(0, 0, 84, 24)
		a.screen.Draw(sim)

		l := calculateLayout(84, 24)
		var row strings.Builder
		for xx := l.ambig.x; xx < l.ambig.x+l.ambig.w; xx++ {
			cell, _, _ := sim.Get(xx, l.ambig.y)
			row.WriteString(cell)
		}
		return row.String()
	}

	off, on := ambigRow(false), ambigRow(true)
	badgeCell := func(row string) int {
		return len([]rune(row[:strings.Index(row, "[E]")]))
	}
	if badgeCell(off) != badgeCell(on) {
		t.Fatalf("toggling E moved its badge: %q vs %q", off, on)
	}
	if !strings.Contains(on, "(I l 1 O 0)") || strings.Index(on, "(I l 1 O 0)") > strings.Index(on, "[E]") {
		t.Fatalf("example should sit before the badge, got %q", on)
	}
}
