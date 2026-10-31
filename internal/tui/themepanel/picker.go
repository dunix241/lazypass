package themepanel

import (
	"fmt"
	"strings"

	"lazypass/internal/debug"
	"lazypass/internal/theme"
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

func (s *Panel) previewTheme(name string) bool {
	p, err := theme.Load(name)
	if err != nil {
		debug.Failure("theme preview load", debug.Invalid)
		s.error = fmt.Sprintf("Cannot load %s: %v", name, err)
		return false
	}
	colors, err := shared.NewPalette(p)
	if err != nil {
		debug.Failure("theme preview palette", debug.Invalid)
		s.error = err.Error()
		return false
	}
	s.pending, s.error = p, ""
	s.setPalette(colors)
	return true
}
func (s *Panel) handleThemePicker(event *tcell.EventKey) {
	if s.filtering {
		s.handleThemeFilter(event)
		return
	}
	switch event.Key() {
	case tcell.KeyUp, tcell.KeyBacktab:
		s.moveThemeSelection(-1)
		return
	case tcell.KeyDown, tcell.KeyTAB:
		s.moveThemeSelection(1)
		return
	case tcell.KeyEnter:
		s.applyThemeSelection()
		return
	}
	if event.Key() != tcell.KeyRune {
		return
	}
	switch event.Rune() {
	case '/':
		s.filtering, s.query = true, ""
	case 'j':
		s.moveThemeSelection(1)
	case 'k':
		s.moveThemeSelection(-1)
	case 'n':
		s.mode, s.name, s.nameEdits, s.error = themeName, "", false, ""
	case 'e':
		if len(s.names) == 0 {
			return
		}
		if theme.IsBuiltin(s.names[s.selected]) {
			s.mode, s.name, s.nameEdits, s.error = themeName, "", false, "Name the editable copy"
			return
		}
		if s.previewTheme(s.names[s.selected]) {
			s.mode, s.role, s.colorEdits = themeEditor, 0, false
			s.colorText = *paletteRole(&s.pending, 0)
		}
	case 'd':
		if len(s.names) > 0 && !theme.IsBuiltin(s.names[s.selected]) {
			s.mode, s.error = themeDeleteConfirm, ""
		}
	}
}
func (s *Panel) matchingThemes() []int {
	matches := make([]int, 0, len(s.names))
	query := strings.ToLower(s.query)
	for i, name := range s.names {
		if !s.filtering || strings.Contains(strings.ToLower(name), query) {
			matches = append(matches, i)
		}
	}
	return matches
}
func (s *Panel) updateThemeFilter(query string) {
	s.query = query
	matches := s.matchingThemes()
	if len(matches) == 0 {
		return
	}
	for _, i := range matches {
		if i == s.selected {
			return
		}
	}
	s.selected = matches[0]
	if s.previewTheme(s.names[s.selected]) {
		s.previewed = true
	}
}
func (s *Panel) handleThemeFilter(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyUp, tcell.KeyDown, tcell.KeyTAB, tcell.KeyBacktab:
		delta := 1
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyBacktab {
			delta = -1
		}
		s.moveThemeSelection(delta)
	case tcell.KeyEnter:
		s.applyThemeSelection()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if s.query != "" {
			runes := []rune(s.query)
			s.updateThemeFilter(string(runes[:len(runes)-1]))
		}
	case tcell.KeyCtrlU:
		s.updateThemeFilter("")
	case tcell.KeyCtrlW:
		s.updateThemeFilter(shared.DeletePreviousWord(s.query))
	case tcell.KeyRune:
		if event.Rune() >= ' ' {
			s.updateThemeFilter(s.query + string(event.Rune()))
		}
	}
}
func (s *Panel) applyThemeSelection() {
	if len(s.matchingThemes()) == 0 || !s.previewTheme(s.names[s.selected]) {
		return
	}
	if err := s.saveTheme(s.names[s.selected]); err != nil {
		debug.Failure("theme apply", debug.IO)
		s.error = "Saving config: " + err.Error()
		return
	}
	s.Close()
}
func (s *Panel) moveThemeSelection(delta int) {
	matches := s.matchingThemes()
	if len(matches) == 0 {
		return
	}
	position := 0
	for i, index := range matches {
		if index == s.selected {
			position = i
			break
		}
	}
	s.selected = matches[(position+delta+len(matches))%len(matches)]
	if s.previewTheme(s.names[s.selected]) {
		s.previewed = true
	}
}
