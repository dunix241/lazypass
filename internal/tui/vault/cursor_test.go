package vault

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
)

func TestBrowserRowCursorReplacesSelectionMarker(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(60, 20)
	view := &View{}
	frame := shared.Frame{Palette: shared.DefaultPalette()}
	view.drawBrowserRow(screen, frame, shared.Rect{X: 2, Y: 2, W: 50, H: 12}, vault.Node{Name: "entry", Kind: vault.EntryNode}, 5, true)
	x, y, visible := screen.GetCursor()
	if !visible || x != 4 || y != 5 {
		t.Fatalf("cursor: %d %d %t", x, y, visible)
	}
	text, _, _ := screen.Get(4, 5)
	if text == ">" {
		t.Fatal("selection marker remains")
	}
	screen.HideCursor()
	view.deleteTarget = &vault.Node{}
	view.drawBrowserRow(screen, frame, shared.Rect{W: 50}, vault.Node{Name: "entry"}, 5, true)
	_, _, visible = screen.GetCursor()
	if visible {
		t.Fatal("delete dialog exposed browser cursor")
	}
}
