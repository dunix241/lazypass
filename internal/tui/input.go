package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (s *screen) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return s.WrapInputHandler(func(event *tcell.EventKey, _ func(tview.Primitive)) {
		if s.overlay == themeOverlay {
			s.themePanel.HandleKey(event)
			return
		}
		if s.overlay == storeOverlay {
			s.store.HandleKey(event)
			return
		}
		if s.route == VaultRoute && s.vault.IsFiltering() {
			s.vault.HandleKey(event)
			return
		}
		if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && event.Rune() == 'q') {
			s.save()
			s.quit()
			return
		}
		if event.Key() == tcell.KeyRune && event.Rune() == 'v' {
			s.switchRoute()
			return
		}
		if s.route == VaultRoute {
			s.vault.HandleKey(event)
			return
		}
		s.generator.HandleKey(event, s.frame())
	})
}
