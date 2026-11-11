package tui

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/config"
	"lazypass/internal/tui/shared"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompactAndKeybindingsFilter(t *testing.T) {
	for _, route := range []Route{GeneratorRoute, VaultRoute} {
		a := NewApp(config.Defaults(), "", nil, WithInitialRoute(route))
		s := a.screen
		press := func(r rune) { s.InputHandler()(tcell.NewEventKey(tcell.KeyRune, r, 0), nil) }
		if strings.Contains(strings.Join(s.footerRows(120), " "), "? keybindings") {
			t.Fatal("normal footer shows help hint")
		}
		press('z')
		if !s.cfg.Compact || strings.TrimSpace(strings.Join(s.footerRows(120), " ")) != "? keybindings" {
			t.Fatal("compact footer incorrect")
		}
		press('?')
		if s.overlay != keybindingsOverlay {
			t.Fatal("help not opened")
		}
		press('/')
		s.PasteHandler()("compact", nil)
		matches := s.keybindings.filteredActions()
		if len(matches) != 1 || matches[0].Hint != "z" {
			t.Fatal("help filter failed")
		}
		s.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), nil)
		if s.overlay != keybindingsOverlay || s.keybindings.filter.Value() != "" {
			t.Fatal("Escape did not clear filter first")
		}
		press('z')
		if s.cfg.Compact {
			t.Fatal("help compact toggle failed")
		}
		s.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), nil)
		if s.overlay != noOverlay {
			t.Fatal("help did not close")
		}
		press('?')
		if s.overlay != keybindingsOverlay {
			t.Fatal("normal-mode help unavailable")
		}
	}
}

func TestCompactPreferencePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewApp(config.Defaults(), path, nil)
	a.screen.toggleCompact()
	loaded, err := config.Load(path)
	if err != nil || !loaded.Compact {
		t.Fatal("compact preference not persisted")
	}
	a.screen.toggleCompact()
	loaded, err = config.Load(path)
	if err != nil || loaded.Compact {
		t.Fatal("normal preference not persisted")
	}
}

func TestConfigSavePreservesPendingCompactPreference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewApp(config.Defaults(), path, nil)
	if err := a.screen.saveOptions(a.screen.generator.Options()); err != nil {
		t.Fatal(err)
	}
	// Simulate a toggle whose background save has not run yet.
	a.screen.cfg.Compact = true
	if err := a.screen.saveOptions(a.screen.generator.Options()); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(path)
	if err != nil || !loaded.Compact || !a.screen.cfg.Compact {
		t.Fatal("config update reverted pending compact preference")
	}
}

func TestKeybindingColumnsRemainAlignedWhenFiltered(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	s := a.screen
	s.keybindings = keybindingsState{actions: []shared.Action{{Hint: "<ctrl+o>", Label: "long key"}, {Hint: "y", Label: "short key"}}}
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(80, 30)
	frame := shared.Frame{Viewport: shared.Rect{W: 80, H: 30}, Palette: shared.DefaultPalette()}
	find := func(text string) (int, int) {
		for y := 0; y < 30; y++ {
			var row strings.Builder
			for x := 0; x < 80; x++ {
				ch, _, _ := sim.Get(x, y)
				row.WriteString(ch)
			}
			if x := strings.Index(row.String(), text); x >= 0 {
				return x, y
			}
		}
		t.Fatalf("text missing: %s", text)
		return 0, 0
	}
	s.drawKeybindings(sim, frame)
	longX, _ := find("long key")
	shortX, _ := find("short key")
	if longX != shortX {
		t.Fatal("description column not aligned")
	}
	s.keybindings.filtering = true
	s.keybindings.paste("short")
	sim.Clear()
	s.drawKeybindings(sim, frame)
	filteredX, _ := find("short key")
	if filteredX != shortX {
		t.Fatal("filter shifted columns")
	}
}
