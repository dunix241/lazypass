package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (s *screen) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return s.WrapInputHandler(func(event *tcell.EventKey, _ func(tview.Primitive)) {
		if s.themePanel.open {
			s.handleThemeInput(event)
			return
		}
		if s.storeForm {
			s.handleStoreInput(event)
			return
		}
		if s.route == VaultRoute && s.vaultFiltering {
			s.handleVaultFilterInput(event)
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
			s.handleVaultInput(event)
			return
		}
		switch event.Key() {
		case tcell.KeyTAB, tcell.KeyDown:
			s.move(1)
			return
		case tcell.KeyBacktab, tcell.KeyUp:
			s.move(-1)
			return
		case tcell.KeyLeft:
			s.selected = focusLength
			s.adjustLength(-1)
			return
		case tcell.KeyRight:
			s.selected = focusLength
			s.adjustLength(1)
			return
		case tcell.KeyEnter:
			s.activate()
			return
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if s.selected == focusLength && len(s.lengthText) > 0 {
				s.lengthText = s.lengthText[:len(s.lengthText)-1]
			}
			return
		}
		if event.Key() != tcell.KeyRune {
			return
		}
		switch event.Rune() {
		case 'U', 'u':
			s.selected = focusUpper
			s.activate()
		case 'L', 'l':
			s.selected = focusLower
			s.activate()
		case 'N', 'n':
			s.selected = focusNumbers
			s.activate()
		case 'S', 's':
			s.selected = focusSymbols
			s.activate()
		case 'a':
			s.openStoreForm()
		case 'E', 'e':
			s.selected = focusAmbiguous
			s.activate()
		case 'j':
			s.move(1)
		case 'k':
			s.move(-1)
		case '-':
			s.selected = focusLength
			s.adjustLength(-1)
		case '+':
			s.selected = focusLength
			s.adjustLength(1)
		case ' ':
			s.activate()
		case 'r':
			_ = s.regenerate()
		case 'c':
			s.copy()
		case 't':
			s.openThemePanel()
		default:
			if s.selected == focusLength && event.Rune() >= '0' && event.Rune() <= '9' {
				s.typeLength(event.Rune())
			}
		}
	})
}
