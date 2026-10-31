package themepanel

import (
	"slices"

	"lazypass/internal/debug"
	"lazypass/internal/theme"
	"lazypass/internal/tui/shared"
)

func (s *Panel) deleteTheme() {
	name := s.names[s.selected]
	if err := theme.Delete(name); err != nil {
		debug.Failure("theme delete", debug.IO)
		s.error, s.mode = err.Error(), themePicker
		return
	}
	names, err := theme.List()
	if err != nil {
		debug.Failure("theme list", debug.IO)
		s.names = append(s.names[:s.selected], s.names[s.selected+1:]...)
		s.error = "Listing themes: " + err.Error()
	} else {
		s.names, s.error = names, ""
	}
	s.mode, s.previewed = themePicker, false
	if s.currentTheme() == name {
		s.setPalette(shared.DefaultPalette())
		fallback := s.deps.UseDefaultTheme
		if fallback == nil {
			fallback = func() error { return s.saveTheme(theme.DefaultName()) }
		}
		if err := fallback(); err != nil {
			debug.Failure("theme fallback save", debug.IO)
			s.error = "Saving config: " + err.Error()
		}
		s.original, s.originalID = shared.DefaultPalette(), theme.DefaultName()
	} else {
		s.setPalette(s.original)
	}
	s.selected = max(0, slices.Index(s.names, s.originalID))
	if pending, err := theme.Load(s.originalID); err == nil {
		s.pending = pending
	} else {
		debug.Failure("theme fallback load", debug.Invalid)
		s.error = "Loading theme: " + err.Error()
	}
}
