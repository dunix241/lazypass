package generator

import (
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

func (s *View) HandleKey(event *tcell.EventKey, frame shared.Frame) {
	switch event.Key() {
	case tcell.KeyTAB, tcell.KeyDown:
		s.move(1, frame)
		return
	case tcell.KeyBacktab, tcell.KeyUp:
		s.move(-1, frame)
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
		if s.deps.OpenStore != nil {
			s.deps.OpenStore()
		}
	case 'E', 'e':
		s.selected = focusAmbiguous
		s.activate()
	case 'j':
		s.move(1, frame)
	case 'k':
		s.move(-1, frame)
	case '-':
		s.selected = focusLength
		s.adjustLength(-1)
	case '+':
		s.selected = focusLength
		s.adjustLength(1)
	case ' ':
		s.activate()
	case 'r':
		_ = s.Regenerate()
	case 'c':
		s.copy()
	case 't':
		if s.deps.OpenThemes != nil {
			s.deps.OpenThemes()
		}
	default:
		if s.selected == focusLength && event.Rune() >= '0' && event.Rune() <= '9' {
			s.typeLength(event.Rune())
		}
	}
}
