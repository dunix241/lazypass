package vault

import (
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (s *StoreForm) HandleMouse(action tview.MouseAction, event *tcell.EventMouse, frame shared.Frame) shared.MouseResult {
	consume := shared.MouseResult{Consumed: true, Capture: true}
	if s.bulk != nil {
		return consume
	}
	if s.loadingEntries || s.saving || s.storeMessage != "" {
		return consume
	}
	if action != tview.MouseLeftDown && action != tview.MouseLeftClick {
		return consume
	}
	x, y := event.Position()
	r, visible := s.storeFormRect(frame.Viewport)
	if s.choosingType {
		return consume
	}
	if s.qrImport || s.qrImage != nil {
		return consume
	}
	if s.editingValue {
		for _, row := range s.fieldRows(r) {
			if x >= r.X+2 && x < r.X+r.W-2 && y >= row.y && y < row.y+row.height {
				s.selectField(row.index)
				break
			}
		}
		return consume
	}
	row := y - (r.Y + 5)
	if x < r.X+2 || x >= r.X+r.W-2 || row < 0 || row >= visible {
		return consume
	}
	start, end := s.suggestionWindow(visible)
	index := start + row
	if index >= end {
		return consume
	}
	if action == tview.MouseLeftClick && index == s.storeSelected {
		s.insertStoreSuggestion()
		return consume
	}
	s.storeSelected = index
	return consume
}
