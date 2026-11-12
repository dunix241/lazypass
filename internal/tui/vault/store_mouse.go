package vault

import (
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (s *StoreForm) HandleMouse(action tview.MouseAction, event *tcell.EventMouse, frame shared.Frame) shared.MouseResult {
	consume := shared.MouseResult{Consumed: true, Capture: true}
	if s.bulk != nil {
		s.handleBulkMouse(action, event, frame)
		return consume
	}
	if s.qrImage != nil || s.qrImport {
		if action == tview.MouseLeftClick {
			r, _ := s.storeFormRect(frame.Viewport)
			if s.qrImage != nil {
				r = s.qrDisplayRect(frame.Viewport)
			}
			if item, ok := popupActionAt(r, s.qrActions(), event); ok {
				s.HandleKey(item.Key)
			}
		}
		return consume
	}
	if s.loadingEntries || s.saving || s.storeMessage != "" {
		return consume
	}
	x, y := event.Position()
	r, visible := s.storeFormRect(frame.Viewport)
	if !r.Contains(x, y) {
		return consume
	}
	if s.choosingType {
		actions := s.createActions()
		switch action {
		case tview.MouseScrollUp:
			s.actionIndex = max(0, s.actionIndex-1)
		case tview.MouseScrollDown:
			s.actionIndex = min(max(0, len(actions)-1), s.actionIndex+1)
		case tview.MouseLeftClick, tview.MouseLeftDoubleClick:
			start, end := s.actionViewport.Window(len(actions), max(0, r.H-4), s.actionIndex)
			index := start + y - r.Y - 2
			if x >= r.X+2 && x < r.X+r.W-2 && y >= r.Y+2 && index >= start && index < end {
				s.actionIndex = index
				if action == tview.MouseLeftDoubleClick {
					s.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
				}
			}
		}
		return consume
	}
	if s.editingValue {
		if action == tview.MouseScrollUp {
			s.selectField(max(0, s.field-1))
			return consume
		}
		if action == tview.MouseScrollDown {
			s.selectField(min(len(s.entryFields())-1, s.field+1))
			return consume
		}
		if action != tview.MouseLeftDown && action != tview.MouseLeftClick {
			return consume
		}
		for _, row := range s.fieldRows(r) {
			if x >= r.X+2 && x < r.X+r.W-2 && y >= row.y && y < row.y+row.height {
				s.selectField(row.index)
				break
			}
		}
		return consume
	}
	switch action {
	case tview.MouseScrollUp:
		s.storeSelected = max(0, s.storeSelected-1)
		return consume
	case tview.MouseScrollDown:
		s.storeSelected = min(max(0, len(s.storeNodes)-1), s.storeSelected+1)
		return consume
	case tview.MouseLeftDown, tview.MouseLeftClick, tview.MouseLeftDoubleClick:
	default:
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
	if action == tview.MouseLeftDoubleClick {
		s.storeSelected = index
		s.insertStoreSuggestion()
		return consume
	}
	s.storeSelected = index
	return consume
}
