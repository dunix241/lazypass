package tui

import "github.com/rivo/tview"

func (s *screen) PasteHandler() func(string, func(tview.Primitive)) {
	return func(text string, _ func(tview.Primitive)) {
		if s.unlock != nil {
			s.unlock.append(text)
			return
		}
		if s.overlay == storeOverlay {
			s.store.Paste(text)
		}
	}
}
