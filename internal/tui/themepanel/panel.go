package themepanel

import (
	"slices"

	"lazypass/internal/debug"
	"lazypass/internal/theme"
	"lazypass/internal/tui/shared"
)

type themePanelMode uint8

const (
	themePicker themePanelMode = iota
	themeName
	themeEditor
	themeDeleteConfirm
)

type Dependencies struct {
	CurrentPalette  func() shared.Palette
	CurrentTheme    func() string
	SetPalette      func(shared.Palette)
	SaveTheme       func(string) error
	UseDefaultTheme func() error
	Notify          func(shared.Level, string)
	Closed          func()
}

type Panel struct {
	open       bool
	mode       themePanelMode
	names      []string
	selected   int
	filtering  bool
	query      string
	original   shared.Palette
	originalID string
	pending    theme.Palette
	name       string
	nameEdits  bool
	role       int
	colorText  string
	colorEdits bool
	previewed  bool
	error      string
	deps       Dependencies
}

func New(deps Dependencies) *Panel { return &Panel{deps: deps} }
func (s *Panel) IsOpen() bool      { return s.open }
func (s *Panel) Close() {
	s.open = false
	if s.deps.Closed != nil {
		s.deps.Closed()
	}
}
func (s *Panel) currentPalette() shared.Palette {
	if s.deps.CurrentPalette != nil {
		return s.deps.CurrentPalette()
	}
	return shared.DefaultPalette()
}
func (s *Panel) currentTheme() string {
	if s.deps.CurrentTheme != nil {
		return s.deps.CurrentTheme()
	}
	return theme.DefaultName()
}
func (s *Panel) setPalette(colors shared.Palette) {
	if s.deps.SetPalette != nil {
		s.deps.SetPalette(colors)
	}
}
func (s *Panel) saveTheme(name string) error {
	if s.deps.SaveTheme != nil {
		return s.deps.SaveTheme(name)
	}
	return nil
}

func (s *Panel) Open() bool {
	names, err := theme.List()
	if err != nil {
		debug.Failure("theme list", debug.IO)
		if s.deps.Notify != nil {
			s.deps.Notify(shared.Error, "Unable to list themes")
		}
		return false
	}
	*s = Panel{open: true, names: names, original: s.currentPalette(), originalID: s.currentTheme(), deps: s.deps}
	s.selected = max(0, slices.Index(names, s.originalID))
	if len(names) > 0 {
		s.previewTheme(names[s.selected])
	}
	return true
}
