package themepanel

import (
	"fmt"
	"slices"
	"strings"

	"lazypass/internal/theme"
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

func (s *Panel) Draw(screen tcell.Screen, frame shared.Frame) {
	viewport, colors := frame.Viewport, frame.Palette
	w, h := min(64, viewport.W-2), min(15, viewport.H-2)
	if w < 28 || h < 8 {
		return
	}
	r := shared.Rect{X: viewport.X + (viewport.W-w)/2, Y: viewport.Y + (viewport.H-h)/2, W: w, H: h}
	shared.DrawBox(screen, r, colors.Border, colors.Surface)
	shared.PrintAt(screen, r.X+2, r.Y, " Theme ", colors.Accent)
	footer := ""
	switch s.mode {
	case themeName:
		shared.PrintAt(screen, r.X+2, r.Y+2, "Custom theme name:", colors.Text)
		shared.PrintAt(screen, r.X+2, r.Y+4, shared.Truncate("["+s.name+"]", w-4), colors.Focus)
		footer = "<enter> create"
	case themeEditor:
		shared.PrintAt(screen, r.X+2, r.Y+2, shared.Truncate("Edit "+s.names[s.selected], w-4), colors.Text)
		rows := min(len(themeRoles), h-5)
		start := max(0, s.role-rows+1)
		for row := 0; row < rows; row++ {
			i := start + row
			value, color := *paletteRole(&s.pending, i), colors.Muted
			if i == s.role {
				value, color = "["+s.colorText+"]", colors.Focus
			}
			shared.PrintAt(screen, r.X+2, r.Y+3+row, shared.Truncate(fmt.Sprintf("%-8s %s", themeRoles[i], value), w-4), color)
		}
		footer = themeHelp(w-4, "<enter> validate  s save", "s save")
	case themeDeleteConfirm:
		shared.PrintAt(screen, r.X+2, r.Y+3, shared.Truncate("Delete "+s.names[s.selected]+"? (y/n)", w-4), colors.Warning)
	default:
		if s.filtering {
			shared.PrintAt(screen, r.X+2, r.Y+2, shared.Truncate("Filter: "+s.query+"_", w-4), colors.Focus)
		} else {
			footer = themePickerHelp(w - 4)
		}
		matches := s.matchingThemes()
		listY, limit := r.Y+2, h-5
		if s.filtering {
			listY, limit = r.Y+4, h-7
		}
		rows := min(len(matches), limit)
		position := max(0, slices.Index(matches, s.selected))
		start := max(0, position-rows+1)
		for i := 0; i < rows; i++ {
			index := matches[start+i]
			prefix, color := "  ", colors.Text
			if index == s.selected {
				prefix, color = "> ", colors.Focus
			}
			kind := "custom"
			if theme.IsBuiltin(s.names[index]) {
				kind = "built-in"
			}
			shared.PrintAt(screen, r.X+2, listY+i, shared.Truncate(prefix+s.names[index]+" ("+kind+")", w-4), color)
		}
		if s.filtering && len(matches) == 0 {
			shared.PrintAt(screen, r.X+2, listY, "No matching themes", colors.Muted)
		}
	}
	if s.error != "" {
		shared.PrintPanelFooter(screen, r, strings.ReplaceAll(s.error, "\n", " "), colors.Warning)
	} else if footer != "" {
		shared.PrintPanelFooter(screen, r, footer, colors.Muted)
	}
}
