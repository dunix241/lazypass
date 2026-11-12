package vault

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"image"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"testing"
)

func clickPopupAction(t *testing.T, r shared.Rect, actions []shared.Action, hint string, click func(*tcell.EventMouse)) {
	t.Helper()
	bounds := popupFooterBounds(r)
	for x := bounds.X; x < bounds.X+bounds.W; x++ {
		e := tcell.NewEventMouse(x, bounds.Y, tcell.Button1, 0)
		if item, ok := popupActionAt(r, actions, e); ok && item.Hint == hint {
			click(e)
			return
		}
	}
	t.Fatalf("missing popup action: %s", hint)
}

func TestConflictAndQRMouseActions(t *testing.T) {
	frame := testFrame()
	frame.Viewport = shared.Rect{W: 84, H: 24}
	v := New(Dependencies{})
	v.transfer = transferState{conflict: true, nodes: []vault.Node{{Name: "fixture"}}, names: []string{"fixture"}}
	r := shared.CenteredPanel(frame.Viewport, 64, 8)
	clickPopupAction(t, r, v.conflictActions(), "r", func(e *tcell.EventMouse) { v.HandleMouse(tview.MouseLeftClick, e, frame) })
	if !v.transfer.rename {
		t.Fatal("conflict rename click ignored")
	}
	clickPopupAction(t, r, v.conflictActions(), "<esc>", func(e *tcell.EventMouse) { v.HandleMouse(tview.MouseLeftClick, e, frame) })
	if v.transfer.conflict {
		t.Fatal("conflict cancel click ignored")
	}
	s := NewStoreForm(StoreDependencies{})
	s.qrImage = image.NewGray(image.Rect(0, 0, 40, 40))
	r = s.qrDisplayRect(frame.Viewport)
	clickPopupAction(t, r, s.qrActions(), "s", func(e *tcell.EventMouse) { s.HandleMouse(tview.MouseLeftClick, e, frame) })
	if !s.qrExport {
		t.Fatal("QR export click ignored")
	}
	clickPopupAction(t, r, s.qrActions(), "<esc>", func(e *tcell.EventMouse) { s.HandleMouse(tview.MouseLeftClick, e, frame) })
	if s.qrExport {
		t.Fatal("QR export back ignored")
	}
}

func TestBulkMouseListFocusToggleAndScroll(t *testing.T) {
	s := NewStoreForm(StoreDependencies{})
	s.bulk = &bulkImport{phase: "preview", rows: make([]bulkRow, 20)}
	frame := testFrame()
	frame.Viewport = shared.Rect{W: 84, H: 24}
	r := bulkRect(frame.Viewport)
	y := bulkListY(r, "preview")
	e := tcell.NewEventMouse(r.X+2, y+1, tcell.Button1, 0)
	s.HandleMouse(tview.MouseLeftClick, e, frame)
	if s.bulk.index != 1 || s.bulk.rows[1].selected {
		t.Fatal("single click did not focus only")
	}
	s.HandleMouse(tview.MouseLeftDoubleClick, e, frame)
	if !s.bulk.rows[1].selected {
		t.Fatal("double click did not toggle account")
	}
	s.HandleMouse(tview.MouseScrollDown, e, frame)
	if s.bulk.index != 2 {
		t.Fatal("wheel did not move selection")
	}
}
