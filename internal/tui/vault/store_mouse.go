package vault

import (
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (s *StoreForm) HandleMouse(action tview.MouseAction, event *tcell.EventMouse, frame shared.Frame) shared.MouseResult {
	consume := shared.MouseResult{Consumed: true, Capture: true}
	if s.editingValue {
		return consume
	}
	if action != tview.MouseLeftDown && action != tview.MouseLeftClick {
		return consume
	}
	x, y := event.Position()
	r, visible := s.storeFormRect(frame.Viewport)
	row := y - (r.Y + 6)
	if x < r.X+2 || x >= r.X+r.W-2 || row < 0 || row >= visible {
		return consume
	}
	if action == tview.MouseLeftClick && row == s.storeSelected {
		s.insertStoreSuggestion()
		return consume
	}
	s.storeSelected = row
	return consume
}
