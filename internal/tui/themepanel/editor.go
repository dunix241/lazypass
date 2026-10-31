package themepanel

import (
	"slices"
	"unicode/utf8"

	"lazypass/internal/debug"
	"lazypass/internal/theme"
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

var themeRoles = []string{"base", "surface", "border", "text", "muted", "accent", "focus", "warning"}

func (s *Panel) handleThemeName(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyCtrlU:
		s.name, s.nameEdits = "", true
	case tcell.KeyCtrlW:
		s.name, s.nameEdits = shared.DeletePreviousWord(s.name), true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(s.name) > 0 {
			runes := []rune(s.name)
			s.name = string(runes[:len(runes)-1])
		}
	case tcell.KeyEnter:
		if err := theme.ValidateName(s.name); err != nil {
			debug.Failure("theme name", debug.Invalid)
			s.error = err.Error()
			return
		}
		if err := theme.Save(s.name, s.pending); err != nil {
			debug.Failure("theme create", debug.IO)
			s.error = err.Error()
			return
		}
		names, err := theme.List()
		if err != nil {
			debug.Failure("theme list", debug.IO)
			s.error = "Listing themes: " + err.Error()
			return
		}
		s.names = names
		s.selected = slices.Index(s.names, s.name)
		if s.selected < 0 {
			debug.Failure("theme create", debug.NotFound)
			s.selected, s.error = 0, "Created theme is missing from the list"
			return
		}
		s.error = ""
		s.mode, s.role, s.colorEdits, s.colorText = themeEditor, 0, false, *paletteRole(&s.pending, 0)
	default:
		if event.Key() == tcell.KeyRune && utf8.RuneCountInString(s.name) < 48 {
			if !s.nameEdits {
				s.name, s.nameEdits = "", true
			}
			s.name += string(event.Rune())
		}
	}
}

func (s *Panel) handleThemeEditor(event *tcell.EventKey) {
	if event.Key() == tcell.KeyRune && event.Rune() == 's' && !s.colorEdits {
		if err := theme.Save(s.names[s.selected], s.pending); err != nil {
			debug.Failure("theme save", debug.IO)
			s.error = err.Error()
		} else {
			s.error = "Saved"
		}
		return
	}
	switch event.Key() {
	case tcell.KeyUp:
		s.role = (s.role - 1 + len(themeRoles)) % len(themeRoles)
		s.colorText, s.colorEdits = *paletteRole(&s.pending, s.role), false
	case tcell.KeyDown:
		s.role = (s.role + 1) % len(themeRoles)
		s.colorText, s.colorEdits = *paletteRole(&s.pending, s.role), false
	case tcell.KeyCtrlU:
		s.colorText, s.colorEdits = "", true
	case tcell.KeyCtrlW:
		s.colorText, s.colorEdits = shared.DeletePreviousWord(s.colorText), true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(s.colorText) > 0 {
			s.colorText = s.colorText[:len(s.colorText)-1]
			s.colorEdits = true
		}
	case tcell.KeyEnter:
		candidate := s.pending
		*paletteRole(&candidate, s.role) = s.colorText
		if err := candidate.Validate(); err != nil {
			debug.Failure("theme color", debug.Invalid)
			s.error = err.Error()
			return
		}
		s.pending, s.error, s.colorEdits = candidate, "", false
		colors, _ := shared.NewPalette(candidate)
		s.setPalette(colors)
		s.previewed = true
	default:
		if event.Key() == tcell.KeyRune && (!s.colorEdits || utf8.RuneCountInString(s.colorText) < 7) {
			if !s.colorEdits {
				s.colorText, s.colorEdits = "", true
			}
			s.colorText += string(event.Rune())
		}
	}
}

func paletteRole(p *theme.Palette, role int) *string {
	switch role {
	case 0:
		return &p.Base
	case 1:
		return &p.Surface
	case 2:
		return &p.Border
	case 3:
		return &p.Text
	case 4:
		return &p.Muted
	case 5:
		return &p.Accent
	case 6:
		return &p.Focus
	case 7:
		return &p.Warning
	}
	return nil
}
