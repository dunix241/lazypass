package shared

import (
	"fmt"
	"strconv"

	"lazypass/internal/theme"

	"github.com/gdamore/tcell/v2"
)

type Palette struct {
	Base, Surface, Border, Text, Muted, Accent, Focus, Warning tcell.Color
}

func NewPalette(p theme.Palette) (Palette, error) {
	values := []string{p.Base, p.Surface, p.Border, p.Text, p.Muted, p.Accent, p.Focus, p.Warning}
	var colors [8]tcell.Color
	for i, value := range values {
		if len(value) != 7 || value[0] != '#' {
			return Palette{}, fmt.Errorf("invalid color %q", value)
		}
		n, err := strconv.ParseInt(value[1:], 16, 32)
		if err != nil {
			return Palette{}, err
		}
		colors[i] = tcell.NewHexColor(int32(n))
	}
	return Palette{colors[0], colors[1], colors[2], colors[3], colors[4], colors[5], colors[6], colors[7]}, nil
}

func DefaultPalette() Palette {
	p, _ := theme.Load(theme.DefaultName())
	colors, _ := NewPalette(p)
	return colors
}
