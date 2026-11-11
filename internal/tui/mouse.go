package tui

import (
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (s *screen) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return s.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, _ func(tview.Primitive)) (bool, tview.Primitive) {
		x, y := event.Position()
		if action == tview.MouseLeftClick {
			for i, bounds := range s.notificationBounds {
				if bounds.Contains(x, y) && s.activateNotification(i) {
					return true, nil
				}
			}
		}
		if s.unlock != nil {
			return true, s
		}
		if s.overlay == vaultPickerOverlay {
			s.handleVaultPickerMouse(action, event)
			return true, s
		}
		if s.overlay == keybindingsOverlay {
			s.handleKeybindingsMouse(action, event)
			return true, s
		}
		if s.overlay == themeOverlay {
			s.themePanel.HandleMouse(action, event, s.frame())
			return true, s
		}
		frame := s.frame()
		var result shared.MouseResult
		if s.overlay == storeOverlay {
			result = s.store.HandleMouse(action, event, frame)
		} else {
			x0, y0, w0, h0 := s.GetRect()
			if calculateLayout(w0, h0).tooSmall {
				return true, s
			}
			footer := s.footerActions()
			rows := s.footerRows(w0)
			if (s.route != VaultRoute || !s.vault.IsConfirmingDelete()) && y >= y0+h0-len(rows) {
				if action == tview.MouseLeftClick {
					if item, ok := footer.HitAction(shared.Rect{X: x0 + 1, Y: y0 + h0 - len(rows), W: max(0, w0-2), H: len(rows)}, shared.FooterOptions{MaxRows: footerMaxRows}, x, y); ok {
						if item.OnClick != nil {
							item.OnClick()
						} else if item.Key != nil {
							s.InputHandler()(item.Key, nil)
						} else if item.Invoke != nil {
							item.Invoke()
						}
					}
				}
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
