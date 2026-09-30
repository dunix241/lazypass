package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"lazypass/internal/config"
	"lazypass/internal/theme"

	"github.com/gdamore/tcell/v2"
)

func themeKey(a *App, key tcell.Key, r rune) {
	a.screen.InputHandler()(tcell.NewEventKey(key, r, tcell.ModNone), nil)
}

func panelRowText(sim tcell.SimulationScreen, x, y, width int) string {
	var row strings.Builder
	for xx := x; xx < x+width; xx++ {
		cell, _, _ := sim.Get(xx, y)
		row.WriteString(cell)
	}
	return row.String()
}

func TestConfiguredThemeAndFallbackAreScreenLocal(t *testing.T) {
	nord := config.Defaults()
	nord.Theme = "nord"
	a := NewApp(nord, "", nil)
	if a.screen.colors == defaultTUIPalette() {
		t.Fatal("nord should not use the default palette")
	}

	bad := config.Defaults()
	bad.Theme = "missing-theme"
	b := NewApp(bad, "", nil)
	if b.screen.colors != defaultTUIPalette() {
		t.Fatal("missing configured theme should use Midnight Rose")
	}
	if b.screen.status == "" {
		t.Fatal("missing configured theme should report a nonfatal status")
	}
	if a.screen.colors == b.screen.colors {
		t.Fatal("theme palettes must belong to individual screens")
	}
}

func TestThemePickerPreviewApplyAndCancelPreservesGeneratorFocus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewApp(config.Defaults(), path, nil)
	a.screen.selected = focusSymbols
	themeKey(a, tcell.KeyRune, 't')
	if !a.screen.themePanel.open || a.screen.selected != focusSymbols {
		t.Fatal("theme picker should be modal without changing generator focus")
	}
	themeKey(a, tcell.KeyDown, 0)
	if a.screen.colors == defaultTUIPalette() {
		t.Fatal("moving the picker should live-preview the selected palette")
	}
	themeKey(a, tcell.KeyEscape, 0)
	if a.screen.colors != defaultTUIPalette() || !a.screen.themePanel.open {
		t.Fatal("first Escape should discard the preview and retain the panel")
	}
	themeKey(a, tcell.KeyDown, 0)
	themeKey(a, tcell.KeyEnter, 0)
	if a.screen.themePanel.open || a.screen.cfg.Theme != "mono" {
		t.Fatalf("picker apply failed: %+v", a.screen.themePanel)
	}
	loaded, err := config.Load(path)
	if err != nil || loaded.Theme != "mono" {
		t.Fatalf("applied theme was not persisted: %+v, %v", loaded, err)
	}
}

func TestThemePickerVimNavigation(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	themeKey(a, tcell.KeyRune, 't')
	p := &a.screen.themePanel
	start := p.selected
	themeKey(a, tcell.KeyRune, 'j')
	if p.selected != (start+1)%len(p.names) || !p.previewed {
		t.Fatalf("j did not preview next theme: selected=%d previewed=%t", p.selected, p.previewed)
	}
	themeKey(a, tcell.KeyRune, 'k')
	if p.selected != start {
		t.Fatalf("k did not return to original theme: selected=%d", p.selected)
	}
	themeKey(a, tcell.KeyRune, 'n')
	themeKey(a, tcell.KeyRune, 'j')
	themeKey(a, tcell.KeyRune, 'k')
	if p.name != "jk" {
		t.Fatalf("j/k should type in the theme name field, got %q", p.name)
	}
}

func TestThemePickerSlashFilterAndEscape(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	themeKey(a, tcell.KeyRune, 't')
	themeKey(a, tcell.KeyRune, '/')
	p := &a.screen.themePanel
	for _, r := range "drac" {
		themeKey(a, tcell.KeyRune, r)
	}
	if !p.filtering || p.query != "drac" || len(p.matchingThemes()) != 1 || p.names[p.selected] != "dracula" {
		t.Fatalf("filter=%q selected=%q matches=%v", p.query, p.names[p.selected], p.matchingThemes())
	}
	themeKey(a, tcell.KeyRune, 'z')
	if len(p.matchingThemes()) != 0 {
		t.Fatalf("unexpected matches for %q", p.query)
	}
	themeKey(a, tcell.KeyEnter, 0)
	if !p.open || a.screen.cfg.Theme != theme.DefaultName() {
		t.Fatal("Enter with no matches should not apply a theme")
	}
	themeKey(a, tcell.KeyBackspace, 0)
	if p.query != "drac" || len(p.matchingThemes()) != 1 {
		t.Fatalf("Backspace did not restore match: %q", p.query)
	}
	themeKey(a, tcell.KeyEscape, 0)
	if p.filtering || p.query != "" || !p.open || len(p.matchingThemes()) != len(p.names) {
		t.Fatal("Esc should leave filter mode and restore the full list")
	}
	themeKey(a, tcell.KeyRune, 'n')
	if p.mode != themeName {
		t.Fatal("picker shortcuts should work after leaving filter mode")
	}
}

func TestThemePickerFilterCanApply(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewApp(config.Defaults(), path, nil)
	themeKey(a, tcell.KeyRune, 't')
	themeKey(a, tcell.KeyRune, '/')
	for _, r := range "rose" {
		themeKey(a, tcell.KeyRune, r)
	}
	p := &a.screen.themePanel
	if len(p.matchingThemes()) != 2 {
		t.Fatalf("expected both rose themes: %v", p.matchingThemes())
	}
	themeKey(a, tcell.KeyUp, 0)
	if p.names[p.selected] != "rose-pine" {
		t.Fatalf("filtered selection did not wrap: %q", p.names[p.selected])
	}
	themeKey(a, tcell.KeyDown, 0)
	themeKey(a, tcell.KeyDown, 0)
	themeKey(a, tcell.KeyEnter, 0)
	if p.open || a.screen.cfg.Theme != "rose-pine" {
		t.Fatalf("filter selection was not applied: %q", a.screen.cfg.Theme)
	}
}

func TestThemePickerQClosesAndRestoresPreview(t *testing.T) {
	for _, filtered := range []bool{false, true} {
		a := NewApp(config.Defaults(), "", nil)
		original := a.screen.colors
		themeKey(a, tcell.KeyRune, 't')
		if filtered {
			themeKey(a, tcell.KeyRune, '/')
			for _, r := range "nord" {
				themeKey(a, tcell.KeyRune, r)
			}
		} else {
			themeKey(a, tcell.KeyDown, 0)
		}
		if a.screen.colors == original {
			t.Fatal("expected a changed preview")
		}
		if filtered {
			themeKey(a, tcell.KeyRune, 'q')
			if !a.screen.themePanel.open || a.screen.themePanel.query != "nordq" {
				t.Fatal("q should type into the active filter")
			}
			themeKey(a, tcell.KeyEscape, 0)
			if !a.screen.themePanel.open || a.screen.themePanel.filtering {
				t.Fatal("Esc should leave the filter without closing the picker")
			}
		}
		themeKey(a, tcell.KeyRune, 'q')
		if a.screen.themePanel.open || a.screen.colors != original {
			t.Fatalf("q did not close picker and restore theme (filtered=%t)", filtered)
		}
	}
	a := NewApp(config.Defaults(), "", nil)
	themeKey(a, tcell.KeyRune, 't')
	themeKey(a, tcell.KeyRune, 'n')
	themeKey(a, tcell.KeyRune, 'q')
	if !a.screen.themePanel.open || a.screen.themePanel.name != "q" {
		t.Fatal("q should remain text in the theme-name input")
	}
}

func TestThemeEditorCreatesEditsAndDeletesCustomTheme(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := NewApp(config.Defaults(), filepath.Join(t.TempDir(), "config.yaml"), nil)
	themeKey(a, tcell.KeyRune, 't')
	themeKey(a, tcell.KeyRune, 'n')
	for _, r := range "my-theme" {
		themeKey(a, tcell.KeyRune, r)
	}
	themeKey(a, tcell.KeyEnter, 0)
	if a.screen.themePanel.mode != themeEditor {
		t.Fatal("new custom theme should open the editor")
	}
	for _, r := range "#112233" {
		themeKey(a, tcell.KeyRune, r)
	}
	themeKey(a, tcell.KeyEnter, 0)
	themeKey(a, tcell.KeyRune, 's')
	p, err := theme.Load("my-theme")
	if err != nil || p.Base != "#112233" {
		t.Fatalf("custom edit was not saved: %+v, %v", p, err)
	}
	themeKey(a, tcell.KeyEscape, 0)
	themeKey(a, tcell.KeyRune, 'd')
	themeKey(a, tcell.KeyRune, 'y')
	if _, err := theme.Load("my-theme"); err == nil {
		t.Fatal("confirmed delete did not remove custom theme")
	}
}

func TestDeletingAppliedThemePersistsFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	palette, err := theme.Load("nord")
	if err != nil {
		t.Fatal(err)
	}
	if err := theme.Save("mine", palette); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.Defaults()
	cfg.Theme = "mine"
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	a := NewApp(cfg, path, nil)
	themeKey(a, tcell.KeyRune, 't')
	themeKey(a, tcell.KeyRune, 'd')
	themeKey(a, tcell.KeyRune, 'y')
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Theme != theme.DefaultName() || a.screen.cfg.Theme != theme.DefaultName() || a.screen.colors != defaultTUIPalette() {
		t.Fatalf("deleted theme was not replaced: saved=%q current=%q", loaded.Theme, a.screen.cfg.Theme)
	}
	themeKey(a, tcell.KeyEscape, 0)
	if a.screen.colors != defaultTUIPalette() {
		t.Fatal("Escape restored the deleted theme")
	}
}

func TestThemePanelErrorStaysInsideBorder(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	a.screen.openThemePanel()
	a.screen.themePanel.error = "Theme failed"
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(80, 24)
	a.screen.SetRect(0, 0, 80, 24)
	a.screen.Draw(sim)
	px, py := (80-64)/2, (24-15)/2
	footerRow := panelRowText(sim, px, py+15-2, 64)
	border, _, _ := sim.Get(px+2, py+15-1)
	if border != "─" || !strings.Contains(footerRow, "Theme failed") {
		t.Fatalf("error overlapped the panel border: border=%q row=%q", border, footerRow)
	}
}

func TestThemePanelRendersInsideWideAndCompactViewports(t *testing.T) {
	for _, size := range [][2]int{{84, 24}, {50, 18}, {32, 18}} {
		a := NewApp(config.Defaults(), "", nil)
		a.screen.openThemePanel()
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(size[0], size[1])
		a.screen.SetRect(0, 0, size[0], size[1])
		a.screen.Draw(sim)
		w, h := min(64, size[0]-2), min(15, size[1]-2)
		x, y := (size[0]-w)/2+2, (size[1]-h)/2
		name, _, _ := sim.Get(x+2, y+2)
		if name == " " {
			t.Fatalf("%dx%d expected theme names above footer hints", size[0], size[1])
		}
		px := (size[0] - w) / 2
		footerRow := panelRowText(sim, px+1, y+h-2, w-2)
		hint := themePickerHelp(w - 4)
		leading := strings.Index(footerRow, hint)
		imbalance := w - 2 - utf8.RuneCountInString(hint) - 2*leading
		if leading < 1 || imbalance < 0 || imbalance > 1 {
			t.Fatalf("%dx%d expected centered footer hints, got %q", size[0], size[1], footerRow)
		}
		var cells strings.Builder
		for y := 0; y < size[1]; y++ {
			for x := 0; x < size[0]; x++ {
				cell, _, _ := sim.Get(x, y)
				cells.WriteString(cell)
			}
		}
		sim.Fini()
		if !strings.Contains(cells.String(), "Theme") || !strings.Contains(cells.String(), "/ filter") || strings.Contains(cells.String(), "<esc> close") {
			t.Fatalf("%dx%d did not render the theme picker hints", size[0], size[1])
		}
		if size[0] >= 50 && !strings.Contains(cells.String(), "n new  e edit  d delete  / filter") {
			t.Fatalf("%dx%d should render full picker actions", size[0], size[1])
		}
	}
}
