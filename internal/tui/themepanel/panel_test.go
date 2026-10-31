package themepanel

import (
	"errors"
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/theme"
	"lazypass/internal/tui/shared"
	"strings"
	"testing"
	"unicode/utf8"
)

type host struct {
	colors    shared.Palette
	name      string
	saveError error
}

func newPanel() (*Panel, *host) {
	h := &host{colors: shared.DefaultPalette(), name: theme.DefaultName()}
	p := New(Dependencies{CurrentPalette: func() shared.Palette { return h.colors }, CurrentTheme: func() string { return h.name }, SetPalette: func(colors shared.Palette) { h.colors = colors }, SaveTheme: func(name string) error {
		if h.saveError != nil {
			return h.saveError
		}
		h.name = name
		return nil
	}})
	return p, h
}
func key(p *Panel, k tcell.Key, r rune) { p.HandleKey(tcell.NewEventKey(k, r, tcell.ModNone)) }
func rowText(sim tcell.SimulationScreen, x, y, width int) string {
	var row strings.Builder
	for xx := x; xx < x+width; xx++ {
		cell, _, _ := sim.Get(xx, y)
		row.WriteString(cell)
	}
	return row.String()
}

func TestThemePickerVimNavigation(t *testing.T) {
	p, _ := newPanel()
	p.Open()
	start := p.selected
	key(p, tcell.KeyRune, 'j')
	if p.selected != (start+1)%len(p.names) || !p.previewed {
		t.Fatal("j did not preview next theme")
	}
	key(p, tcell.KeyRune, 'k')
	if p.selected != start {
		t.Fatal("k did not return to original")
	}
	for _, r := range "njk" {
		key(p, tcell.KeyRune, r)
	}
	if p.name != "jk" {
		t.Fatal("name input consumed j/k as shortcuts")
	}
}

func TestThemePickerSlashFilterAndEscape(t *testing.T) {
	p, h := newPanel()
	p.Open()
	for _, r := range "/drac" {
		key(p, tcell.KeyRune, r)
	}
	if !p.filtering || p.query != "drac" || len(p.matchingThemes()) != 1 || p.names[p.selected] != "dracula" {
		t.Fatal("filter failed")
	}
	key(p, tcell.KeyRune, 'z')
	key(p, tcell.KeyEnter, 0)
	if len(p.matchingThemes()) != 0 || !p.IsOpen() || h.name != theme.DefaultName() {
		t.Fatal("empty filter applied a theme")
	}
	key(p, tcell.KeyBackspace, 0)
	if p.query != "drac" || len(p.matchingThemes()) != 1 {
		t.Fatal("Backspace did not restore match")
	}
	key(p, tcell.KeyEscape, 0)
	if p.filtering || p.query != "" || !p.IsOpen() || len(p.matchingThemes()) != len(p.names) {
		t.Fatal("Escape did not leave filter")
	}
	key(p, tcell.KeyRune, 'n')
	if p.mode != themeName {
		t.Fatal("shortcuts did not resume")
	}
}

func TestThemePickerFilterCanApply(t *testing.T) {
	p, h := newPanel()
	p.Open()
	for _, r := range "/rose" {
		key(p, tcell.KeyRune, r)
	}
	if len(p.matchingThemes()) != 2 {
		t.Fatal("rose filter mismatch")
	}
	key(p, tcell.KeyUp, 0)
	if p.names[p.selected] != "rose-pine" {
		t.Fatal("filtered selection did not wrap")
	}
	key(p, tcell.KeyDown, 0)
	key(p, tcell.KeyDown, 0)
	key(p, tcell.KeyEnter, 0)
	if p.IsOpen() || h.name != "rose-pine" {
		t.Fatal("filtered apply failed")
	}
}

func TestThemePickerQClosesAndRestoresPreview(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		p, h := newPanel()
		original := h.colors
		p.Open()
		if filtered {
			for _, r := range "/nord" {
				key(p, tcell.KeyRune, r)
			}
		} else {
			key(p, tcell.KeyDown, 0)
		}
		if h.colors == original {
			t.Fatal("expected changed preview")
		}
		if filtered {
			key(p, tcell.KeyRune, 'q')
			if !p.IsOpen() || p.query != "nordq" {
				t.Fatal("q did not type in filter")
			}
			key(p, tcell.KeyEscape, 0)
			if !p.IsOpen() || p.filtering {
				t.Fatal("Escape closed filter incorrectly")
			}
		}
		key(p, tcell.KeyRune, 'q')
		if p.IsOpen() || h.colors != original {
			t.Fatal("q did not restore/close")
		}
	}
	p, _ := newPanel()
	p.Open()
	for _, r := range "nq" {
		key(p, tcell.KeyRune, r)
	}
	if !p.IsOpen() || p.name != "q" {
		t.Fatal("q did not remain text in name input")
	}
}

func TestThemeApplyFailureRetainsPanel(t *testing.T) {
	p, h := newPanel()
	p.Open()
	h.saveError = errors.New("config unavailable")
	key(p, tcell.KeyDown, 0)
	key(p, tcell.KeyEnter, 0)
	if !p.IsOpen() || p.error == "" || h.name != theme.DefaultName() {
		t.Fatal("failed apply closed/persisted theme")
	}
	key(p, tcell.KeyRune, 'q')
	if h.colors != shared.DefaultPalette() {
		t.Fatal("failed apply could not cancel")
	}
}

func TestThemeEditorCreatesEditsAndDeletesCustomTheme(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p, _ := newPanel()
	p.Open()
	for _, r := range "nmy-theme" {
		key(p, tcell.KeyRune, r)
	}
	key(p, tcell.KeyEnter, 0)
	if p.mode != themeEditor {
		t.Fatal("create did not open editor")
	}
	for _, r := range "#112233" {
		key(p, tcell.KeyRune, r)
	}
	key(p, tcell.KeyEnter, 0)
	key(p, tcell.KeyRune, 's')
	palette, err := theme.Load("my-theme")
	if err != nil || palette.Base != "#112233" {
		t.Fatal("custom palette not saved")
	}
	key(p, tcell.KeyEscape, 0)
	key(p, tcell.KeyRune, 'd')
	key(p, tcell.KeyRune, 'y')
	if _, err := theme.Load("my-theme"); err == nil {
		t.Fatal("confirmed deletion failed")
	}
}

func TestThemePanelErrorStaysInsideBorder(t *testing.T) {
	p, h := newPanel()
	p.Open()
	p.error = "Theme failed"
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(80, 24)
	p.Draw(sim, shared.Frame{Viewport: shared.Rect{W: 80, H: 24}, Palette: h.colors})
	px, py := (80-64)/2, (24-15)/2
	footer := rowText(sim, px, py+13, 64)
	border, _, _ := sim.Get(px+2, py+14)
	if border != "─" || !strings.Contains(footer, "Theme failed") {
		t.Fatal("error overlaps panel border")
	}
}

func TestThemePanelRendersInsideWideAndCompactViewports(t *testing.T) {
	for _, size := range [][2]int{{84, 24}, {50, 18}, {32, 18}} {
		p, h := newPanel()
		p.Open()
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(size[0], size[1])
		p.Draw(sim, shared.Frame{Viewport: shared.Rect{W: size[0], H: size[1]}, Palette: h.colors})
		w, hh := min(64, size[0]-2), min(15, size[1]-2)
		px, py := (size[0]-w)/2, (size[1]-hh)/2
		name, _, _ := sim.Get(px+4, py+2)
		if name == " " {
			t.Fatal("theme names missing")
		}
		footer := rowText(sim, px+1, py+hh-2, w-2)
		hint := themePickerHelp(w - 4)
		leading := strings.Index(footer, hint)
		imbalance := w - 2 - utf8.RuneCountInString(hint) - 2*leading
		if leading < 1 || imbalance < 0 || imbalance > 1 {
			t.Fatalf("footer not centered: %q", footer)
		}
		var cells strings.Builder
		for y := 0; y < size[1]; y++ {
			cells.WriteString(rowText(sim, 0, y, size[0]))
		}
		sim.Fini()
		if !strings.Contains(cells.String(), "Theme") || !strings.Contains(cells.String(), "/ filter") || strings.Contains(cells.String(), "<esc> close") {
			t.Fatal("picker hints missing")
		}
		if size[0] >= 50 && !strings.Contains(cells.String(), "n new  e edit  d delete  / filter") {
			t.Fatal("full picker actions missing")
		}
	}
}
