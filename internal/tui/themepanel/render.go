package themepanel

import (
	"fmt"
	"strings"

	"lazypass/internal/theme"
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

func (s *Panel) Draw(screen tcell.Screen, frame shared.Frame) {
	colors := frame.Palette
	r := themePanelRect(frame.Viewport)
	w, h := r.W, r.H
	if w < 28 || h < 8 {
		return
	}
	border := colors.Border
	if s.mode == themeDeleteConfirm {
		border = tcell.ColorRed
	}
	shared.Panel{Bounds: r, Title: "Theme", TitleColor: colors.Accent, Border: border, Background: colors.Surface}.Draw(screen, func(content shared.Rect) {
		switch s.mode {
		case themeName:
			shared.PrintAt(screen, r.X+2, r.Y+2, "Custom theme name:", colors.Text)
			shared.PrintAt(screen, r.X+2, r.Y+4, shared.Truncate("["+s.name+"]", w-4), colors.Focus)
		case themeEditor:
			shared.PrintAt(screen, r.X+2, r.Y+2, shared.Truncate("Edit "+s.names[s.selected], w-4), colors.Text)
			rows := min(len(themeRoles), h-5)
			start, _ := s.roleViewport.Window(len(themeRoles), rows, s.role)
			for row := 0; row < rows; row++ {
				i := start + row
				value, color := *paletteRole(&s.pending, i), colors.Muted
				if i == s.role {
					value, color = "["+s.colorText+"]", colors.Focus
				}
				shared.PrintAt(screen, r.X+2, r.Y+3+row, shared.Truncate(fmt.Sprintf("%-8s %s", themeRoles[i], value), w-4), color)
			}
		case themeDeleteConfirm:
			shared.PrintAt(screen, r.X+2, r.Y+3, shared.Truncate("Delete "+s.names[s.selected]+"? (y/n)", w-4), colors.Warning)
		default:
			if s.filtering {
				shared.PrintAt(screen, r.X+2, r.Y+2, shared.Truncate("Filter: "+s.query+"_", w-4), colors.Focus)
			}
			matches, listY, start, end := s.pickerWindow(r)
			for i := start; i < end; i++ {
				index := matches[i]
				prefix, color := "  ", colors.Text
				if index == s.selected {
					prefix, color = "> ", colors.Focus
				}
				kind := "custom"
				if theme.IsBuiltin(s.names[index]) {
					kind = "built-in"
				}
				shared.PrintAt(screen, r.X+2, listY+i-start, shared.Truncate(prefix+s.names[index]+" ("+kind+")", w-4), color)
			}
			if s.filtering && len(matches) == 0 {
				shared.PrintAt(screen, r.X+2, listY, "No matching themes", colors.Muted)
			}
		}
		if s.error != "" {
			shared.PrintPanelFooter(screen, r, strings.ReplaceAll(s.error, "\n", " "), colors.Warning)
		} else {
			rows := (shared.Footer{Actions: s.footerActions()}).Rows(r.W-4, shared.FooterOptions{MaxRows: 1})
			if len(rows) > 0 {
				shared.PrintPanelFooter(screen, r, rows[0], colors.Muted)
			}
		}
	})
}
