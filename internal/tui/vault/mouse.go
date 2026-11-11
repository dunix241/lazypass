package vault

import (
	"lazypass/internal/tui/shared"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (s *View) HandleMouse(action tview.MouseAction, event *tcell.EventMouse, frame shared.Frame) shared.MouseResult {
	if s.IsConfirmingDelete() || s.IsTransferActive() {
		return shared.MouseResult{Consumed: true, Capture: true}
	}
	consume := shared.MouseResult{Consumed: true, Capture: true}
	switch action {
	case tview.MouseScrollUp:
		s.moveVaultSelection(-1)
		return consume
	case tview.MouseScrollDown:
		s.moveVaultSelection(1)
		return consume
	case tview.MouseRightClick, tview.MouseRightDoubleClick:
		s.vaultParent()
		return consume
	case tview.MouseLeftDown, tview.MouseLeftUp, tview.MouseMove:
		return consume
	case tview.MouseLeftDoubleClick, tview.MouseLeftClick:
		x, y := event.Position()
		index := s.vaultRowAt(x, y, frame.Card)
		if index < 0 {
			return shared.MouseResult{}
		}
		chip, code, _ := s.authenticatorChip(s.vaultDisplayNodes()[index], frame.Card, time.Now())
		if code != "" && code != "Loading…" && x >= chip.X && x < chip.X+chip.W {
			s.vaultSelected = index
			s.copyEntry(true)
			return consume
		}
		activate := action == tview.MouseLeftDoubleClick || index == s.vaultSelected
		s.vaultSelected = index
		if activate {
			s.openVaultNode()
		}
		return consume
	default:
		return shared.MouseResult{}
	}
}

func (s *View) vaultRowAt(x, y int, r shared.Rect) int {
	if s.vault.Provider == nil {
		return -1
	}
	nodes := s.vaultDisplayNodes()
	if len(nodes) == 0 || x < r.X+1 || x >= r.X+r.W-1 {
		return -1
	}
	listY, limit, start := s.vaultListWindow(r)
	offset := y - listY
	if limit <= 0 || offset < 0 || offset >= limit {
		return -1
	}
	index := start + offset
	if index < 0 || index >= len(nodes) {
		return -1
	}
	return index
}
