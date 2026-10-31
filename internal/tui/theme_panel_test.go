package tui

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/config"
	"lazypass/internal/theme"
	"lazypass/internal/tui/shared"
	"path/filepath"
	"testing"
)

func themeKey(a *App, key tcell.Key, r rune) {
	a.screen.InputHandler()(tcell.NewEventKey(key, r, tcell.ModNone), nil)
}

func TestConfiguredThemeAndFallbackAreScreenLocal(t *testing.T) {
	nord := config.Defaults()
	nord.Theme = "nord"
	a := NewApp(nord, "", nil)
	bad := config.Defaults()
	bad.Theme = "missing-theme"
	b := NewApp(bad, "", nil)
	if a.screen.colors == shared.DefaultPalette() || b.screen.colors != shared.DefaultPalette() || b.screen.status == "" || a.screen.colors == b.screen.colors {
		t.Fatal("screen-local configured theme/fallback failed")
	}
}

func TestThemePickerPreviewApplyAndCancelPreservesGeneratorFocus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewApp(config.Defaults(), path, nil)
	themeKey(a, tcell.KeyRune, 's')
	before := a.screen.generator.Footer(120)
	themeKey(a, tcell.KeyRune, 't')
	if !a.screen.themePanel.IsOpen() || a.screen.generator.Footer(120) != before {
		t.Fatal("picker changed generator focus")
	}
	themeKey(a, tcell.KeyDown, 0)
	if a.screen.colors == shared.DefaultPalette() {
		t.Fatal("preview not propagated")
	}
	themeKey(a, tcell.KeyEscape, 0)
	if a.screen.colors != shared.DefaultPalette() || !a.screen.themePanel.IsOpen() {
		t.Fatal("preview not restored")
	}
	themeKey(a, tcell.KeyDown, 0)
	themeKey(a, tcell.KeyEnter, 0)
	loaded, err := config.Load(path)
	if err != nil || loaded.Theme != "mono" || a.screen.cfg.Theme != "mono" || a.screen.themePanel.IsOpen() {
		t.Fatalf("apply failed: %+v %v", loaded, err)
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
	for _, r := range "tdy" {
		themeKey(a, tcell.KeyRune, r)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Theme != theme.DefaultName() || a.screen.cfg.Theme != theme.DefaultName() || a.screen.colors != shared.DefaultPalette() {
		t.Fatal("fallback not persisted")
	}
	themeKey(a, tcell.KeyEscape, 0)
	if a.screen.colors != shared.DefaultPalette() {
		t.Fatal("cancel restored deleted theme")
	}
}
