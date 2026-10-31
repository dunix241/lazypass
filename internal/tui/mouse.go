package tui

import (
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (s *screen) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return s.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, _ func(tview.Primitive)) (bool, tview.Primitive) {
		if s.overlay == themeOverlay {
			return true, s
		}
		frame := s.frame()
		var result shared.MouseResult
		if s.overlay == storeOverlay {
			result = s.store.HandleMouse(action, event, frame)
		} else {
			_, _, w, h := s.GetRect()
			if calculateLayout(w, h).tooSmall {
				return true, s
			}
			if s.route == VaultRoute {
				result = s.vault.HandleMouse(action, event, frame)
			} else {
				result = s.generator.HandleMouse(action, event, frame)
			}
		}
		if result.Capture {
			return result.Consumed, s
		}
		return result.Consumed, nil
	})
}
