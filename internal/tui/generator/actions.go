package generator

import (
	"strconv"

	"lazypass/internal/app"
	"lazypass/internal/debug"
	"lazypass/internal/generator"
	"lazypass/internal/tui/shared"
)

func (s *View) Regenerate() error {
	results, err := app.Generate(s.cfg, 1)
	if err != nil {
		debug.Failure("generator regenerate", debug.Unknown)
		return err
	}
	s.password, s.bits, s.strength = results[0].Password, results[0].EntropyBits, results[0].Strength
	return nil
}

func (s *View) move(delta int, frame shared.Frame) {
	for {
		s.selected = focus((int(s.selected) + delta + int(focusCount)) % int(focusCount))
		if !frame.Compact || s.selected != focusRegen {
			return
		}
	}
}
func (s *View) adjustLength(delta int) { s.setLength(s.cfg.Length + delta) }
func (s *View) typeLength(r rune) {
	if len(s.lengthText) >= 3 {
		s.lengthText = ""
	}
	s.lengthText += string(r)
	if n, err := strconv.Atoi(s.lengthText); err == nil && n >= generator.MinLength && n <= generator.MaxLength {
		s.setLength(n)
	}
}
func (s *View) setLength(n int) {
	n = max(generator.MinLength, min(generator.MaxLength, n))
	s.cfg.Length, s.lengthText = n, strconv.Itoa(n)
	_ = s.Regenerate()
	s.save()
}

func (s *View) activate() {
	previous := s.cfg
	switch s.selected {
	case focusUpper:
		s.cfg.Upper = !s.cfg.Upper
	case focusLower:
		s.cfg.Lower = !s.cfg.Lower
	case focusNumbers:
		s.cfg.Numbers = !s.cfg.Numbers
	case focusSymbols:
		s.cfg.Symbols = !s.cfg.Symbols
	case focusAmbiguous:
		s.cfg.ExcludeAmbiguous = !s.cfg.ExcludeAmbiguous
	case focusStore:
		if s.deps.OpenStore != nil {
			s.deps.OpenStore()
		}
		return
	case focusRegen:
		_ = s.Regenerate()
		return
	case focusCopy:
		s.copy()
		return
	case focusLength:
		return
	}
	if !s.cfg.Upper && !s.cfg.Lower && !s.cfg.Numbers && !s.cfg.Symbols {
		s.cfg = previous
		s.notify(shared.Warning, "Enable at least one character type")
		return
	}
	_ = s.Regenerate()
	s.save()
}

func (s *View) copy() {
	if s.deps.Copy != nil {
		if err := s.deps.Copy(s.password); err != nil {
			debug.Failure("generator copy", debug.Clipboard)
			s.notify(shared.Error, "Clipboard unavailable")
			return
		}
	}
	s.notify(shared.Success, "Copied to clipboard")
}
