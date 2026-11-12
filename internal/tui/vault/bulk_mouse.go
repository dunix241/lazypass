package vault

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"lazypass/internal/tui/shared"
)

func bulkRect(viewport shared.Rect) shared.Rect {
	return shared.CenteredPanel(viewport, min(76, viewport.W-4), min(20, viewport.H-4))
}

func bulkListY(r shared.Rect, phase string) int {
	step, title := bulkStep(phase)
	height := 1
	if r.W < 72 {
		height = len(shared.WrapText(fmt.Sprintf("Step %d of 4 · %s", step, title), r.W-4))
	}
	y := r.Y + 3 + height
	if phase == "destination" {
		y += 2
	}
	return y
}

func (s *StoreForm) handleBulkMouse(action tview.MouseAction, event *tcell.EventMouse, frame shared.Frame) {
	b := s.bulk
	r := bulkRect(frame.Viewport)
	x, y := event.Position()
	if !r.Contains(x, y) {
		return
	}
	if action == tview.MouseLeftClick {
		if item, ok := popupActionAt(r, s.bulkActions(), event); ok {
			s.handleBulkKey(item.Key)
			return
		}
	}
	if s.saving {
		return
	}
	if b.phase != "preview" && b.phase != "results" && b.phase != "destination" {
		return
	}
	count, index := len(b.rows), &b.index
	viewport := &b.listViewport
	if b.phase == "destination" {
		count, index = len(b.folders)+1, &b.folderIndex
		viewport = &b.destinationViewport
	}
	switch action {
	case tview.MouseScrollUp:
		*index = max(0, *index-1)
		return
	case tview.MouseScrollDown:
		*index = min(max(0, count-1), *index+1)
		return
	case tview.MouseLeftClick, tview.MouseLeftDoubleClick:
	default:
		return
	}
	listY := bulkListY(r, b.phase)
	start, end := viewport.Window(count, max(1, r.Y+r.H-2-listY), *index)
	row := start + y - listY
	if x < r.X+2 || x >= r.X+r.W-2 || y < listY || row < start || row >= end {
		return
	}
	*index = row
	if action == tview.MouseLeftDoubleClick {
		if b.phase == "destination" {
			s.handleBulkKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
		}
		if b.phase == "preview" {
			s.handleBulkKey(tcell.NewEventKey(tcell.KeyRune, ' ', 0))
		}
	}
}
