package generator

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"lazypass/internal/generator"
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

func (s *View) Draw(screen tcell.Screen, frame shared.Frame) {
	l := calculateLayout(frame)
	colors := frame.Palette
	r := l.card
	shared.Panel{Bounds: r.shared(), Title: "Generate a password", TitleColor: colors.Accent, Border: colors.Border, Background: colors.Surface}.Draw(screen, nil)
	passwordLabelY, passwordY, meterY, lengthLabelY, charactersY := r.y+2, r.y+3, r.y+4, r.y+6, r.y+9
	if l.short && !l.compact {
		passwordLabelY, passwordY, meterY, lengthLabelY, charactersY = r.y+1, r.y+2, r.y+3, r.y+5, r.y+8
	}
	shared.PrintAt(screen, r.x+2, passwordLabelY, "PASSWORD", colors.Muted)
	shared.PrintAt(screen, r.x+2, passwordY, shared.Truncate(s.password, max(4, l.store.x-r.x-3)), colors.Text)
	s.drawAction(screen, l.store, colors, focusStore, "+ Add")
	if !l.compact {
		s.drawAction(screen, l.regen, colors, focusRegen, "↻ Regenerate")
	}
	s.drawAction(screen, l.copy, colors, focusCopy, "⧉ Copy")
	blocks := min(8, int(s.bits/16))
	meter := ""
	for i := 0; i < 8; i++ {
		if i < blocks {
			meter += "█"
		} else {
			meter += "░"
		}
	}
	shared.PrintAt(screen, r.x+2, meterY, meter, colors.Accent)
	shared.PrintAt(screen, r.x+12, meterY, fmt.Sprintf("%d bits  %s", int(s.bits+.5), s.strength), colors.Muted)
	lengthValue, lengthLabelColor := s.lengthText, colors.Muted
	if s.selected == focusLength {
		lengthValue = "[" + lengthValue + "]"
		lengthLabelColor = colors.Focus
	}
	shared.PrintAt(screen, r.x+2, lengthLabelY, "Length: "+lengthValue, lengthLabelColor)
	s.drawLength(screen, l.length, colors)
	if !l.compact {
		shared.PrintAt(screen, r.x+2, charactersY, "Characters", colors.Muted)
	}
	s.drawCheck(screen, l.upper, colors, focusUpper, "Uppercase", s.cfg.Upper)
	s.drawCheck(screen, l.lower, colors, focusLower, "Lowercase", s.cfg.Lower)
	s.drawCheck(screen, l.numbers, colors, focusNumbers, "Numbers", s.cfg.Numbers)
	s.drawCheck(screen, l.symbols, colors, focusSymbols, "Symbols", s.cfg.Symbols)
	ambigLabel := "Exclude ambiguous (I l 1 O 0)"
	if l.compact {
		ambigLabel = "Exclude ambiguous"
	}
	s.drawCheck(screen, l.ambig, colors, focusAmbiguous, ambigLabel, s.cfg.ExcludeAmbiguous)
}

func (s *View) drawAction(screen tcell.Screen, r rect, colors shared.Palette, f focus, label string) {
	foreground, background := colors.Text, colors.Border
	if s.selected == f {
		foreground, background = colors.Base, colors.Focus
	}
	style := tcell.StyleDefault.Foreground(foreground).Background(background).Bold(true)
	for column := r.x; column < r.x+r.w; column++ {
		screen.SetContent(column, r.y, ' ', nil, style)
	}
	shared.PrintStyled(screen, r.x+1, r.y, label, style)
}

func (s *View) drawLength(screen tcell.Screen, r rect, colors shared.Palette) {
	color := colors.Text
	if s.selected == focusLength {
		color = colors.Focus
	}
	shared.PrintAt(screen, r.x, r.y, "4", colors.Muted)
	railStart, railWidth := sliderRail(r)
	lengthValue := s.cfg.Length
	if parsed, err := strconv.Atoi(s.lengthText); err == nil {
		lengthValue = max(generator.MinLength, min(generator.MaxLength, parsed))
	}
	knob := railStart + (lengthValue-generator.MinLength)*(railWidth-1)/(generator.MaxLength-generator.MinLength)
	for i := 0; i < railWidth; i++ {
		ch := '─'
		if railStart+i <= knob {
			ch = '━'
		}
		screen.SetContent(railStart+i, r.y, ch, nil, tcell.StyleDefault.Foreground(color).Background(colors.Surface))
	}
	screen.SetContent(knob, r.y, '●', nil, tcell.StyleDefault.Foreground(color).Background(colors.Surface))
	shared.PrintAt(screen, railStart+railWidth+1, r.y, "256", colors.Muted)
}

func (s *View) drawCheck(screen tcell.Screen, r rect, colors shared.Palette, f focus, label string, checked bool) {
	mark, color := "[ ]", colors.Text
	if checked {
		mark = "[✓]"
	}
	if s.selected == f {
		color = colors.Focus
	}
	shared.PrintAt(screen, r.x, r.y, mark+" "+label, color)
	initialX := r.x + 4
	cell, style, _ := screen.Get(initialX, r.y)
	initial, size := utf8.DecodeRuneInString(cell)
	var combining []rune
	if size < len(cell) {
		combining = []rune(cell[size:])
	}
	screen.SetContent(initialX, r.y, initial, combining, style.Underline(true))
}
