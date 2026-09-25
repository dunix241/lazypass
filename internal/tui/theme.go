package tui

import (
	"fmt"
	"strconv"

	"lazypass/internal/theme"

	"github.com/gdamore/tcell/v2"
)

type tuiPalette struct {
	base, surface, border, text, muted, accent, focus, warning tcell.Color
}

func newTUIPalette(p theme.Palette) (tuiPalette, error) {
	color := func(value string) (tcell.Color, error) {
		if len(value) != 7 || value[0] != '#' {
			return tcell.ColorDefault, fmt.Errorf("invalid color %q", value)
		}
		n, err := strconv.ParseInt(value[1:], 16, 32)
		if err != nil {
			return tcell.ColorDefault, err
		}
		return tcell.NewHexColor(int32(n)), nil
	}
	base, err := color(p.Base)
	if err != nil {
		return tuiPalette{}, err
	}
	surface, err := color(p.Surface)
	if err != nil {
		return tuiPalette{}, err
	}
	border, err := color(p.Border)
	if err != nil {
		return tuiPalette{}, err
	}
	text, err := color(p.Text)
	if err != nil {
		return tuiPalette{}, err
	}
	muted, err := color(p.Muted)
	if err != nil {
		return tuiPalette{}, err
	}
	accent, err := color(p.Accent)
	if err != nil {
		return tuiPalette{}, err
	}
	focus, err := color(p.Focus)
	if err != nil {
		return tuiPalette{}, err
	}
	warning, err := color(p.Warning)
	if err != nil {
		return tuiPalette{}, err
	}
	return tuiPalette{base, surface, border, text, muted, accent, focus, warning}, nil
}

func defaultTUIPalette() tuiPalette {
	p, _ := theme.Load(theme.DefaultName())
	colors, _ := newTUIPalette(p)
	return colors
}
