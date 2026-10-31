package shared

import (
	"testing"

	"lazypass/internal/theme"
)

func TestPaletteRejectsSignedHexColors(t *testing.T) {
	p, err := theme.Load(theme.DefaultName())
	if err != nil {
		t.Fatal(err)
	}
	for _, color := range []string{"#+12345", "#-12345", "#gggggg", "#12345"} {
		p.Base = color
		if _, err := NewPalette(p); err == nil {
			t.Fatalf("accepted invalid color %q", color)
		}
	}
}
