package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"lazypass/internal/config"

	"github.com/gdamore/tcell/v2"
)

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

func TestSavePreservesLatestNonGeneratorConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	current := config.Defaults()
	current.Vault = config.Vault{Provider: "pass", StoreDir: "/vault"}
	current.NerdFont = false
	if err := current.Save(path); err != nil {
		t.Fatal(err)
	}
	a := NewApp(config.Defaults(), path, nil)
	a.screen.cfg.Length = 32
	a.screen.save()
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Length != 32 || loaded.Vault != current.Vault || loaded.NerdFont != current.NerdFont {
		t.Fatalf("saved config = %#v", loaded)
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

func TestDrawUnderlinesOptionShortcuts(t *testing.T) {
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
	for _, option := range []struct {
		r       rect
		initial rune
	}{
		{l.upper, 'U'}, {l.lower, 'L'}, {l.numbers, 'N'}, {l.symbols, 'S'}, {l.ambig, 'E'},
	} {
		cell, style, _ := sim.Get(option.r.x+4, option.r.y)
		initial, _ := utf8.DecodeRuneInString(cell)
		_, _, attrs := style.Decompose()
		if initial != option.initial || attrs&tcell.AttrUnderline == 0 {
			t.Fatalf("shortcut at %v = %q, attrs %v", option.r, initial, attrs)
		}
		var row strings.Builder
		for xx := option.r.x + 4; xx < option.r.x+option.r.w; xx++ {
			cell, _, _ := sim.Get(xx, option.r.y)
			row.WriteString(cell)
		}
		if strings.Contains(row.String(), "[") {
			t.Fatalf("shortcut badge still rendered in %q", row.String())
		}
	}
	for _, r := range []rect{l.upper, l.symbols} {
		_, style, _ := sim.Get(r.x+4, r.y)
		foreground, _, _ := style.Decompose()
		if foreground != a.screen.colors.text {
			t.Fatalf("enabled and disabled options should share resting color: %v", foreground)
		}
	}
	a.screen.selected = focusSymbols
	a.screen.Draw(sim)
	_, style, _ := sim.Get(l.symbols.x+4, l.symbols.y)
	foreground, _, attrs := style.Decompose()
	if foreground != a.screen.colors.focus || attrs&tcell.AttrUnderline == 0 {
		t.Fatalf("focused shortcut lost focus color or underline: color %v, attrs %v", foreground, attrs)
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

func TestToggleKeepsShortcutUnderlineAligned(t *testing.T) {
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
		cell, style, _ := sim.Get(l.ambig.x+4, l.ambig.y)
		initial, _ := utf8.DecodeRuneInString(cell)
		_, _, attrs := style.Decompose()
		if initial != 'E' || attrs&tcell.AttrUnderline == 0 {
			t.Fatalf("shortcut not underlined when exclude=%t: %q, attrs %v", exclude, initial, attrs)
		}
		var row strings.Builder
		for xx := l.ambig.x; xx < l.ambig.x+l.ambig.w; xx++ {
			cell, _, _ := sim.Get(xx, l.ambig.y)
			row.WriteString(cell)
		}
		return row.String()
	}

	off, on := ambigRow(false), ambigRow(true)
	if !strings.Contains(off, "[ ] Exclude ambiguous (I l 1 O 0)") || !strings.Contains(on, "[✓] Exclude ambiguous (I l 1 O 0)") {
		t.Fatalf("option label shifted when toggled: %q vs %q", off, on)
	}
}
