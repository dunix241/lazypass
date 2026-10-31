package shared

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

func PrintAt(screen tcell.Screen, x, y int, text string, color tcell.Color) {
	tview.Print(screen, tview.Escape(text), x, y, len(text), tview.AlignLeft, color)
}

func Truncate(value string, width int) string {
	if uniseg.StringWidth(value) <= width {
		return value
	}
	if width < 2 {
		return ""
	}
	var result strings.Builder
	for _, r := range value {
		if uniseg.StringWidth(result.String()+string(r)) > width-1 {
			break
		}
		result.WriteRune(r)
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
	PrintAt(screen, r.X+max(1, (r.W-utf8.RuneCountInString(text))/2), r.Y+r.H-2, text, color)
}
