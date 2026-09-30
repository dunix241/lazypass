package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazypass/internal/config"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

type memoryVault struct {
	nodes map[string][]vault.Node
	read  map[string]vault.Entry
	store vault.Entry
	err   error
}

type safeDiagnosticError struct{}

func (safeDiagnosticError) Error() string       { return "secret-value in external error" }
func (safeDiagnosticError) Unwrap() error       { return vault.ErrLocked }
func (safeDiagnosticError) UserMessage() string { return "GPG: no terminal for pinentry" }

func (m *memoryVault) Name() string                     { return "memory" }
func (m *memoryVault) Capabilities() vault.Capabilities { return vault.Capabilities{} }
func (m *memoryVault) List(_ context.Context, path vault.Path) ([]vault.Node, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.nodes[path.String()], nil
}
func (m *memoryVault) Read(_ context.Context, path vault.Path) (vault.Entry, error) {
	return m.read[path.String()], m.err
}
func (m *memoryVault) Write(_ context.Context, entry vault.Entry) error {
	m.store = entry
	return m.err
}
func (m *memoryVault) Delete(context.Context, vault.Path) error { return nil }
func (m *memoryVault) Sync(context.Context) (vault.SyncResult, error) {
	return vault.SyncResult{}, nil
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

func TestRouteSwitchPreservesGeneratorState(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil, WithInitialRoute(VaultRoute))
	if a.CurrentRoute() != VaultRoute {
		t.Fatal("initial route should be vault")
	}
	a.screen.switchRoute()
	a.screen.selected = focusSymbols
	a.screen.switchRoute()
	a.screen.switchRoute()
	if a.CurrentRoute() != GeneratorRoute || a.screen.selected != focusSymbols {
		t.Fatal("generator state should survive route switching")
	}
}

func TestResizeMessageMatchesMinimumLayout(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil, WithInitialRoute(VaultRoute))
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(32, 17)
	a.screen.SetRect(0, 0, 32, 17)
	a.screen.Draw(sim)
	var row strings.Builder
	for x := 0; x < 32; x++ {
		cell, _, _ := sim.Get(x, 8)
		row.WriteString(cell)
	}
	if !strings.Contains(row.String(), "min 32x18") {
		t.Fatalf("resize message = %q", row.String())
	}
}
