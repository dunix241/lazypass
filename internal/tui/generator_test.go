package tui

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/config"
	"path/filepath"
	"testing"
)

func TestTUIOptionChangesPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewApp(config.Defaults(), path, nil)
	for range 2 {
		a.screen.InputHandler()(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone), nil)
	}
	for _, r := range "128se" {
		a.screen.InputHandler()(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone), nil)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Length != 128 || !loaded.Symbols || !loaded.ExcludeAmbiguous {
		t.Fatalf("saved options: %+v", loaded)
	}
}

func TestSavePreservesLatestNonGeneratorConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	current := config.Defaults()
	current.Vault = config.Vault{Provider: "pass", StoreDir: "/vault"}
	current.NerdFont = false
	current.Theme = "nord"
	if err := current.Save(path); err != nil {
		t.Fatal(err)
	}
	a := NewApp(config.Defaults(), path, nil)
	options := a.CurrentOptions()
	options.Length = 32
	if err := a.screen.saveOptions(options); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Length != 32 || loaded.Vault != current.Vault || loaded.NerdFont != current.NerdFont || loaded.Theme != current.Theme {
		t.Fatalf("saved config: %#v", loaded)
	}
}

func TestThemeSaveDoesNotReplaceLiveGeneratorOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewApp(config.Defaults(), path, nil)
	a.screen.InputHandler()(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone), nil)
	before := a.CurrentOptions()
	if err := a.screen.saveTheme("nord"); err != nil {
		t.Fatal(err)
	}
	if a.CurrentOptions() != before {
		t.Fatal("theme save replaced generator state")
	}
}

func TestDrawPaintsEntireViewport(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	if err := a.Init(); err != nil {
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
	_, background, _ := style.Decompose()
	if background != a.screen.colors.Base {
		t.Fatal("viewport background not painted")
	}
	logo := calculateLogoLayout(84, 24)
	cell, _, _ := sim.Get(logo.x, 0)
	if cell != "l" || logo.x != (84-logo.w)/2 {
		t.Fatal("centered wordmark missing")
	}
}
