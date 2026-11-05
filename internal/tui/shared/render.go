package shared

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

func PrintAt(screen tcell.Screen, x, y int, text string, color tcell.Color) {
	tview.Print(screen, tview.Escape(text), x, y, len(text), tview.AlignLeft, color)
}

// PrintStyled writes literal graphemes with a complete style, including background
// and attributes, without reading and rewriting existing screen cells.
func PrintStyled(screen tcell.Screen, x, y int, text string, style tcell.Style) {
	graphemes := uniseg.NewGraphemes(text)
	for graphemes.Next() {
		_, width := screen.Put(x, y, graphemes.Str(), style)
		x += width
	}
}

func Truncate(value string, width int) string {
	if uniseg.StringWidth(value) <= width {
		return value
	}
	if width < 2 {
		return ""
	}
	var result strings.Builder
	used := 0
	graphemes := uniseg.NewGraphemes(value)
	for graphemes.Next() {
		if used+graphemes.Width() > width-1 {
			break
		}
		result.WriteString(graphemes.Str())
		used += graphemes.Width()
	}
	return result.String() + "…"
}

func DrawBox(screen tcell.Screen, r Rect, color, surface tcell.Color) {
	style := tcell.StyleDefault.Foreground(color).Background(surface)
	for yy := r.Y + 1; yy < r.Y+r.H-1; yy++ {
		for xx := r.X + 1; xx < r.X+r.W-1; xx++ {
			screen.SetContent(xx, yy, ' ', nil, style)
		}
	}
	for xx := r.X + 1; xx < r.X+r.W-1; xx++ {
		screen.SetContent(xx, r.Y, '─', nil, style)
		screen.SetContent(xx, r.Y+r.H-1, '─', nil, style)
	}
	for yy := r.Y + 1; yy < r.Y+r.H-1; yy++ {
		screen.SetContent(r.X, yy, '│', nil, style)
		screen.SetContent(r.X+r.W-1, yy, '│', nil, style)
	}
	screen.SetContent(r.X, r.Y, '╭', nil, style)
	screen.SetContent(r.X+r.W-1, r.Y, '╮', nil, style)
	screen.SetContent(r.X, r.Y+r.H-1, '╰', nil, style)
	screen.SetContent(r.X+r.W-1, r.Y+r.H-1, '╯', nil, style)
}

func PrintPanelFooter(screen tcell.Screen, r Rect, text string, color tcell.Color) {
	text = Truncate(text, r.W-4)
	PrintAt(screen, r.X+max(1, (r.W-uniseg.StringWidth(text))/2), r.Y+r.H-2, text, color)
}
