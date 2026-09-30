package tui

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

func printAt(screen tcell.Screen, x, y int, text string, color tcell.Color) {
	tview.Print(screen, tview.Escape(text), x, y, len(text), tview.AlignLeft, color)
}
func truncate(value string, width int) string {
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
func drawBox(screen tcell.Screen, r rect, color, surface tcell.Color) {
	style := tcell.StyleDefault.Foreground(color).Background(surface)
	for yy := r.y + 1; yy < r.y+r.h-1; yy++ {
		for xx := r.x + 1; xx < r.x+r.w-1; xx++ {
			screen.SetContent(xx, yy, ' ', nil, style)
		}
	}
	for xx := r.x + 1; xx < r.x+r.w-1; xx++ {
		screen.SetContent(xx, r.y, '─', nil, style)
		screen.SetContent(xx, r.y+r.h-1, '─', nil, style)
	}
	for yy := r.y + 1; yy < r.y+r.h-1; yy++ {
		screen.SetContent(r.x, yy, '│', nil, style)
		screen.SetContent(r.x+r.w-1, yy, '│', nil, style)
	}
	screen.SetContent(r.x, r.y, '╭', nil, style)
	screen.SetContent(r.x+r.w-1, r.y, '╮', nil, style)
	screen.SetContent(r.x, r.y+r.h-1, '╰', nil, style)
	screen.SetContent(r.x+r.w-1, r.y+r.h-1, '╯', nil, style)
}

func deletePreviousWord(value string) string {
	runes := []rune(value)
	for len(runes) > 0 && unicode.IsSpace(runes[len(runes)-1]) {
		runes = runes[:len(runes)-1]
	}
	for len(runes) > 0 && !unicode.IsSpace(runes[len(runes)-1]) && runes[len(runes)-1] != '/' {
		runes = runes[:len(runes)-1]
	}
	for len(runes) > 0 && unicode.IsSpace(runes[len(runes)-1]) {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}
