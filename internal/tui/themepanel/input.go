package themepanel

import (
	"slices"

	"github.com/gdamore/tcell/v2"
)

func (s *Panel) HandleKey(event *tcell.EventKey) {
	if s.mode == themePicker && !s.filtering && event.Key() == tcell.KeyRune && event.Rune() == 'q' {
		s.setPalette(s.original)
		s.Close()
		return
	}
	if event.Key() == tcell.KeyEscape {
		if s.mode == themePicker && s.filtering {
			s.filtering, s.query = false, ""
			return
		}
		s.escapeThemePanel()
		return
	}
	switch s.mode {
	case themeName:
		s.handleThemeName(event)
	case themeEditor:
		s.handleThemeEditor(event)
	case themeDeleteConfirm:
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'y', 'Y':
				s.deleteTheme()
			case 'n', 'N':
				s.mode = themePicker
			}
		}
	default:
		s.handleThemePicker(event)
	}
}

func (s *Panel) escapeThemePanel() {
	switch s.mode {
	case themeName, themeEditor, themeDeleteConfirm:
		s.mode, s.error, s.colorEdits = themePicker, "", false
	case themePicker:
		if s.previewed {
			s.setPalette(s.original)
			s.previewed = false
			s.selected = max(0, slices.Index(s.names, s.originalID))
			return
		}
		s.Close()
	}
}
