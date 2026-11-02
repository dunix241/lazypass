package vault

import (
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (s *StoreForm) HandleMouse(action tview.MouseAction, event *tcell.EventMouse, frame shared.Frame) shared.MouseResult {
	consume := shared.MouseResult{Consumed: true, Capture: true}
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
	if s.editingValue {
		rows := min(len(s.entryFields()), max(1, r.H-6))
		start := max(0, s.field-rows+1)
		field := y - (r.Y + 4)
		if x >= r.X+2 && x < r.X+r.W-2 && field >= 0 && field < rows {
			s.selectField(start + field)
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
