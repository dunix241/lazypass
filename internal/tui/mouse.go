package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (s *screen) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return s.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, _ func(tview.Primitive)) (bool, tview.Primitive) {
		if s.themePanel.open {
			return true, s
		}
		if s.storeForm {
			return s.handleStoreMouse(action, event)
		}
		x, y := event.Position()
		ox, oy, w, h := s.GetRect()
		l := calculateLayout(w, h)
		if l.tooSmall {
			return true, s
		}
		if s.route == VaultRoute {
			return s.handleVaultMouse(action, event, l, ox, oy)
		}
		if action == tview.MouseMove && s.draggingSlider {
			s.setLengthFromPointer(l.length, ox, oy, x)
			return true, s
		}
		if action == tview.MouseLeftUp {
			s.draggingSlider = false
			return true, nil
		}
		if action != tview.MouseLeftDown && action != tview.MouseLeftClick {
			return false, nil
		}
		r := l.length
		r.x += ox
		r.y += oy
		if action == tview.MouseLeftDown {
			if !r.contains(x, y) {
				return false, nil
			}
			s.selected = focusLength
			s.draggingSlider = true
			s.setLengthFromPointer(l.length, ox, oy, x)
			return true, s
		}
		click := func(r rect, f focus) bool {
			r.x += ox
			r.y += oy
			if r.contains(x, y) {
				s.selected = f
				s.activate()
				return true
			}
			return false
		}
		if click(l.upper, focusUpper) || click(l.lower, focusLower) || click(l.numbers, focusNumbers) || click(l.symbols, focusSymbols) || click(l.ambig, focusAmbiguous) || click(l.store, focusStore) || (!l.compact && click(l.regen, focusRegen)) || click(l.copy, focusCopy) {
			return true, s
		}
		if r.contains(x, y) {
			s.selected = focusLength
			s.setLengthFromPointer(l.length, ox, oy, x)
			return true, s
		}
		return false, nil
	})
}

func (s *screen) handleVaultMouse(action tview.MouseAction, event *tcell.EventMouse, l layout, ox, oy int) (bool, tview.Primitive) {
	s.draggingSlider = false
	switch action {
	case tview.MouseScrollUp:
		s.moveVaultSelection(-1)
		return true, s
	case tview.MouseScrollDown:
		s.moveVaultSelection(1)
		return true, s
	case tview.MouseRightClick, tview.MouseRightDoubleClick:
		s.vaultParent()
		return true, s
	case tview.MouseLeftDown, tview.MouseLeftUp, tview.MouseMove:
		return true, s
	case tview.MouseLeftDoubleClick:
		x, y := event.Position()
		index := s.vaultRowAt(x, y, l, ox, oy)
		if index < 0 {
			return false, nil
		}
		s.vaultSelected = index
		s.openVaultNode()
		return true, s
	case tview.MouseLeftClick:
		x, y := event.Position()
		index := s.vaultRowAt(x, y, l, ox, oy)
		if index < 0 {
			return false, nil
		}
		if index == s.vaultSelected {
			s.openVaultNode()
			return true, s
		}
		s.vaultSelected = index
		return true, s
	default:
		return false, nil
	}
}

func (s *screen) vaultRowAt(x, y int, l layout, ox, oy int) int {
	if s.vault.Provider == nil {
		return -1
	}
	nodes := s.vaultDisplayNodes()
	if len(nodes) == 0 {
		return -1
	}
	r := l.card
	r.x += ox
	r.y += oy
	if x < r.x+1 || x >= r.x+r.w-1 {
		return -1
	}
	listY, limit, start := s.vaultListWindow(r)
	if limit <= 0 {
		return -1
	}
	offset := y - listY
	if offset < 0 || offset >= limit {
		return -1
	}
	index := start + offset
	if index < 0 || index >= len(nodes) {
		return -1
	}
	return index
}

func (s *screen) handleStoreMouse(action tview.MouseAction, event *tcell.EventMouse) (bool, tview.Primitive) {
	if action != tview.MouseLeftDown && action != tview.MouseLeftClick {
		return true, s
	}
	x, y := event.Position()
	ox, oy, width, height := s.GetRect()
	r, visible := s.storeFormRect(ox, oy, width, height)
	row := y - (r.y + 6)
	if x < r.x+2 || x >= r.x+r.w-2 || row < 0 || row >= visible {
		return true, s
	}
	if action == tview.MouseLeftClick && row == s.storeSelected {
		s.insertStoreSuggestion()
		return true, s
	}
	s.storeSelected = row
	return true, s
}
