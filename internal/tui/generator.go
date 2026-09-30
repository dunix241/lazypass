package tui

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"lazypass/internal/app"
	"lazypass/internal/debug"
	"lazypass/internal/generator"

	"github.com/gdamore/tcell/v2"
)

func (s *screen) regenerate() error {
	results, err := app.Generate(s.cfg.ToOptions(), 1)
	if err != nil {
		debug.Failure("generator regenerate", debug.Unknown)
		return err
	}
	s.password, s.bits, s.strength = results[0].Password, results[0].EntropyBits, results[0].Strength
	return nil
}

func (s *screen) drawCard(screen tcell.Screen, l layout, ox, oy int) {
	r := l.card
	r.x += ox
	r.y += oy
	drawBox(screen, r, s.colors.border, s.colors.surface)
	printAt(screen, r.x+2, r.y, " Generate a password ", s.colors.accent)
	passwordLabelY, passwordY, meterY, lengthLabelY, charactersY := r.y+2, r.y+3, r.y+4, r.y+6, r.y+9
	if l.short && !l.compact {
		passwordLabelY, passwordY, meterY, lengthLabelY, charactersY = r.y+1, r.y+2, r.y+3, r.y+5, r.y+8
	}
	printAt(screen, r.x+2, passwordLabelY, "PASSWORD", s.colors.muted)
	pw := truncate(s.password, max(12, r.w-33))
	printAt(screen, r.x+2, passwordY, pw, s.colors.text)
	s.drawAction(screen, l.store, ox, oy, focusStore, "+ Add")
	if !l.compact {
		s.drawAction(screen, l.regen, ox, oy, focusRegen, "↻ Regenerate")
	}
	s.drawAction(screen, l.copy, ox, oy, focusCopy, "⧉ Copy")
	blocks := min(8, int(s.bits/16))
	meter := ""
	for i := 0; i < 8; i++ {
		if i < blocks {
			meter += "█"
		} else {
			meter += "░"
		}
	}
	printAt(screen, r.x+2, meterY, meter, s.colors.accent)
	strength := fmt.Sprintf("%d bits  %s", int(s.bits+.5), s.strength)
	printAt(screen, r.x+12, meterY, strength, s.colors.muted)

	lengthValue := s.lengthText
	lengthLabelColor := s.colors.muted
	if s.selected == focusLength {
		lengthValue = "[" + lengthValue + "]"
		lengthLabelColor = s.colors.focus
	}
	lengthLabel := "Length: " + lengthValue
	printAt(screen, r.x+2, lengthLabelY, lengthLabel, lengthLabelColor)
	s.drawLength(screen, l.length, ox, oy)
	if !l.compact {
		printAt(screen, r.x+2, charactersY, "Characters", s.colors.muted)
	}
	s.drawCheck(screen, l.upper, ox, oy, focusUpper, "Uppercase", s.cfg.Upper)
	s.drawCheck(screen, l.lower, ox, oy, focusLower, "Lowercase", s.cfg.Lower)
	s.drawCheck(screen, l.numbers, ox, oy, focusNumbers, "Numbers", s.cfg.Numbers)
	s.drawCheck(screen, l.symbols, ox, oy, focusSymbols, "Symbols", s.cfg.Symbols)
	ambigLabel := "Exclude ambiguous (I l 1 O 0)"
	if l.compact {
		ambigLabel = "Exclude ambiguous"
	}
	s.drawCheck(screen, l.ambig, ox, oy, focusAmbiguous, ambigLabel, s.cfg.ExcludeAmbiguous)
}

func (s *screen) drawAction(screen tcell.Screen, r rect, ox, oy int, f focus, label string) {
	r.x += ox
	r.y += oy
	color := s.colors.muted
	if s.selected == f {
		color = s.colors.focus
	}
	printAt(screen, r.x, r.y, label, color)
}

func (s *screen) drawLength(screen tcell.Screen, r rect, ox, oy int) {
	r.x += ox
	r.y += oy
	color := s.colors.text
	if s.selected == focusLength {
		color = s.colors.focus
	}
	printAt(screen, r.x, r.y, "4", s.colors.muted)
	railStart, railWidth := sliderRail(r)
	lengthValue := s.cfg.Length
	if parsed, err := strconv.Atoi(s.lengthText); err == nil {
		lengthValue = parsed
	}
	knob := railStart + (lengthValue-generator.MinLength)*(railWidth-1)/(generator.MaxLength-generator.MinLength)
	for i := 0; i < railWidth; i++ {
		ch := '─'
		if railStart+i <= knob {
			ch = '━'
		}
		screen.SetContent(railStart+i, r.y, ch, nil, tcell.StyleDefault.Foreground(color).Background(s.colors.surface))
	}
	screen.SetContent(knob, r.y, '●', nil, tcell.StyleDefault.Foreground(color).Background(s.colors.surface))
	printAt(screen, railStart+railWidth+1, r.y, "256", s.colors.muted)
}

func (s *screen) drawCheck(screen tcell.Screen, r rect, ox, oy int, f focus, label string, checked bool) {
	r.x += ox
	r.y += oy
	mark, color := "[ ]", s.colors.text
	if checked {
		mark = "[✓]"
	}
	if s.selected == f {
		color = s.colors.focus
	}
	printAt(screen, r.x, r.y, mark+" "+label, color)
	initialX := r.x + 4
	cell, style, _ := screen.Get(initialX, r.y)
	initial, size := utf8.DecodeRuneInString(cell)
	var combining []rune
	if size < len(cell) {
		combining = []rune(cell[size:])
	}
	screen.SetContent(initialX, r.y, initial, combining, style.Underline(true))
}

func (s *screen) move(delta int) {
	_, _, width, height := s.GetRect()
	l := calculateLayout(width, height)
	for {
		s.selected = focus((int(s.selected) + delta + int(focusCount)) % int(focusCount))
		if !l.compact || s.selected != focusRegen {
			return
		}
	}
}
func (s *screen) adjustLength(delta int) { s.setLength(s.cfg.Length + delta) }
func (s *screen) typeLength(r rune) {
	if len(s.lengthText) >= 3 {
		s.lengthText = ""
	}
	s.lengthText += string(r)
	if n, err := strconv.Atoi(s.lengthText); err == nil && n >= generator.MinLength && n <= generator.MaxLength {
		s.setLength(n)
	}
}
func (s *screen) setLength(n int) {
	n = max(generator.MinLength, min(generator.MaxLength, n))
	s.cfg.Length, s.lengthText = n, strconv.Itoa(n)
	_ = s.regenerate()
	s.save()
}

func (s *screen) activate() {
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
		s.openStoreForm()
		return
	case focusRegen:
		_ = s.regenerate()
		return
	case focusCopy:
		s.copy()
		return
	case focusLength:
		return
	}
	if !s.cfg.Upper && !s.cfg.Lower && !s.cfg.Numbers && !s.cfg.Symbols {
		switch s.selected {
		case focusUpper:
			s.cfg.Upper = true
		case focusLower:
			s.cfg.Lower = true
		case focusNumbers:
			s.cfg.Numbers = true
		case focusSymbols:
			s.cfg.Symbols = true
		}
		s.notify(notificationWarning, "Enable at least one character type")
		return
	}
	_ = s.regenerate()
	s.save()
}
func (s *screen) copy() {
	if s.onCopy != nil {
		if err := s.onCopy(s.password); err != nil {
			debug.Failure("generator copy", debug.Clipboard)
			s.notify(notificationError, "Clipboard unavailable")
			return
		}
	}
	s.notify(notificationSuccess, "Copied to clipboard")
}

func sliderRail(r rect) (start, width int) { return r.x + 3, max(8, r.w-8) }

func (s *screen) setLengthFromPointer(r rect, ox, oy, pointerX int) {
	r.x += ox
	r.y += oy
	railStart, railWidth := sliderRail(r)
	pointerX = max(railStart, min(railStart+railWidth-1, pointerX))
	n := generator.MinLength + (pointerX-railStart)*(generator.MaxLength-generator.MinLength)/(railWidth-1)
	s.setLength(n)
}
