package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"lazypass/internal/config"
	"lazypass/internal/theme"

	"github.com/gdamore/tcell/v2"
)

func themeKey(a *App, key tcell.Key, r rune) {
	a.screen.InputHandler()(tcell.NewEventKey(key, r, tcell.ModNone), nil)
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

func TestThemePanelRendersInsideWideAndCompactViewports(t *testing.T) {
	for _, size := range [][2]int{{84, 24}, {50, 18}} {
		a := NewApp(config.Defaults(), "", nil)
		a.screen.openThemePanel()
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(size[0], size[1])
		a.screen.SetRect(0, 0, size[0], size[1])
		a.screen.Draw(sim)
		var cells strings.Builder
		for y := 0; y < size[1]; y++ {
			for x := 0; x < size[0]; x++ {
				cell, _, _ := sim.Get(x, y)
				cells.WriteString(cell)
			}
		}
		sim.Fini()
		if !strings.Contains(cells.String(), "Theme") {
			t.Fatalf("%dx%d did not render the theme panel", size[0], size[1])
		}
	}
}
