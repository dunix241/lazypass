package tui

import (
	"os"
	"path/filepath"
	"testing"

	"lazypass/internal/config"
	"lazypass/internal/theme"
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

func TestPersistencePreservesInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	invalid := []byte("length: [broken\n")
	if err := os.WriteFile(path, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	a := NewApp(config.Defaults(), path, nil)
	if err := a.screen.saveOptions(a.CurrentOptions()); err == nil {
		t.Fatal("invalid config save accepted")
	}
	if err := a.screen.saveTheme("nord"); err == nil {
		t.Fatal("invalid config theme save accepted")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != string(invalid) {
		t.Fatal("invalid config overwritten")
	}
}

func TestDeletingAppliedThemeRetainsLiveFallbackWhenConfigInvalid(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	palette, err := theme.Load("nord")
	if err != nil {
		t.Fatal(err)
	}
	if err := theme.Save("mine", palette); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	invalid := []byte("length: [broken\n")
	if err := os.WriteFile(path, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Theme = "mine"
	a := NewApp(cfg, path, nil)
	for _, r := range "tdy" {
		themeKey(a, tcell.KeyRune, r)
	}
	if a.screen.cfg.Theme != theme.DefaultName() || a.screen.colors != shared.DefaultPalette() || !a.screen.themePanel.IsOpen() {
		t.Fatal("failed fallback save lost live fallback/error panel")
	}
	themeKey(a, tcell.KeyRune, 'q')
	if a.screen.colors != shared.DefaultPalette() {
		t.Fatal("close restored deleted theme")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != string(invalid) {
		t.Fatal("invalid config overwritten")
	}
}
