package generator

import (
	"strconv"

	"lazypass/internal/generator"
	"lazypass/internal/tui/shared"
)

type focus int

const (
	focusLength focus = iota
	focusUpper
	focusLower
	focusNumbers
	focusSymbols
	focusAmbiguous
	focusStore
	focusRegen
	focusCopy
	focusCount
)

type Dependencies struct {
	Copy        func(string) error
	SaveOptions func(generator.Options) error
	Notify      func(shared.Level, string)
	OpenStore   func()
	OpenThemes  func()
}

type View struct {
	cfg            generator.Options
	password       string
	bits           float64
	strength       string
	selected       focus
	lengthText     string
	draggingSlider bool
	deps           Dependencies
}

func New(options generator.Options, deps Dependencies) *View {
	return &View{cfg: options, lengthText: strconv.Itoa(options.Length), deps: deps}
}

func (s *View) Password() string           { return s.password }
func (s *View) Options() generator.Options { return s.cfg }
func (s *View) ReleaseMouse()              { s.draggingSlider = false }
func (s *View) notify(level shared.Level, message string) {
	if s.deps.Notify != nil {
		s.deps.Notify(level, message)
	}
}
func (s *View) save() {
	if s.deps.SaveOptions != nil {
		_ = s.deps.SaveOptions(s.cfg)
	}
}
