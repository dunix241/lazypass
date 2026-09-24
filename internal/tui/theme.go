package tui

import "github.com/gdamore/tcell/v2"

var theme = struct {
	base, surface, border, text, muted, accent, focus, warning tcell.Color
}{
	// Midnight Rose is lazypass's signature palette: plum surfaces, rose actions,
	// and lavender focus states. It intentionally avoids the common terminal green.
	base:    tcell.NewRGBColor(21, 17, 31),
	surface: tcell.NewRGBColor(33, 26, 47),
	border:  tcell.NewRGBColor(82, 65, 109),
	text:    tcell.NewRGBColor(244, 239, 250),
	muted:   tcell.NewRGBColor(181, 168, 203),
	accent:  tcell.NewRGBColor(255, 143, 171),
	focus:   tcell.NewRGBColor(202, 157, 246),
	warning: tcell.NewRGBColor(251, 191, 126),
}
