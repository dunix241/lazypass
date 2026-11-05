package vault

import (
	"lazypass/internal/tui/input"
	"lazypass/internal/tui/shared"
	"strings"

	"github.com/gdamore/tcell/v2"
)

func (s *StoreForm) Draw(screen tcell.Screen, frame shared.Frame) {
	screen.HideCursor()
	r, visible := s.storeFormRect(frame.Viewport)
	colors := frame.Palette
	shared.DrawBox(screen, r, colors.Focus, colors.Surface)
	title := " Store generated password "
	if s.manual {
		title = " Add to vault "
	}
	if s.loadingEntry || s.editEntry != nil {
		title = " Edit entry "
	}
	if s.saving && !s.loadingEntry {
		title = strings.TrimSpace(title) + " · working… "
	}
	shared.PrintAt(screen, r.X+2, r.Y, title, colors.Accent)
	if s.loadingEntry {
		shared.PrintAt(screen, r.X+2, r.Y+3, "Loading entry…", colors.Muted)
		return
	}
	if s.choosingType {
		for i, kind := range s.deps.Service.SupportedEntryTypes() {
			color := colors.Text
			prefix := "  "
			if kind == s.entryType {
				color = colors.Focus
				prefix = "> "
			}
			shared.PrintAt(screen, r.X+2, r.Y+2+i, prefix+kind.String(), color)
		}
		shared.PrintPanelFooter(screen, r, "↑/↓ select", colors.Muted)
		return
	}
	if s.loadingEntries {
		shared.PrintAt(screen, r.X+2, r.Y+3, "Loading entries…", colors.Muted)
		return
	}
	if s.editingValue {
		shared.PrintAt(screen, r.X+2, r.Y+2, shared.Truncate("Entry: "+s.storePath, r.W-4), colors.Muted)
		fields := s.entryFields()
		extra := 0
		if fields[s.field].multiline {
			extra = min(3, max(0, r.H-7))
		}
		rows := min(len(fields), max(1, r.H-6-extra))
		start := max(0, s.field-rows+1)
		for row := 0; row < rows; row++ {
			i := start + row
			field := fields[i]
			color := colors.Text
			focused := i == s.field
			if focused {
				color = colors.Focus
			}
			label := field.label + ": "
			y := r.Y + 4 + row
			if i > s.field {
				y += extra
			}
			shared.PrintAt(screen, r.X+2, y, label, color)
			x, width := r.X+2+len(label), r.W-4-len(label)
			options := field.inputOptions()
			style := tcell.StyleDefault.Foreground(color).Background(colors.Surface)
			if focused {
				s.editor(field.key, s.displayField(i)).Draw(screen, x, y, width, 1+extra, style, true, options)
			} else {
				input.Preview(screen, x, y, width, s.displayField(i), style, options)
			}
		}
		hint := "<tab> field"
		if fields[s.field].multiline {
			hint += "  •  <ctrl-n> newline"
		}
		shared.PrintPanelFooter(screen, r, shared.FitHints(r.W-4, hint, "<tab> field"), colors.Muted)
		return
	}
	shared.PrintAt(screen, r.X+2, r.Y+2, "Destination (folder/name):", colors.Muted)
	s.editor("destination", s.storePath).Draw(screen, r.X+2, r.Y+3, r.W-4, 1, tcell.StyleDefault.Foreground(colors.Text).Background(colors.Surface), true, input.Options{MaxBytes: fieldByteLimit})
	if s.storeMessage != "" {
		shared.PrintAt(screen, r.X+2, r.Y+5, shared.Truncate(s.storeMessage, r.W-4), colors.Warning)
	} else if visible > 0 {
		start, end := s.suggestionWindow(visible)
		for index := start; index < end; index++ {
			node := s.storeNodes[index]
			prefix := "  "
			if index == s.storeSelected {
				prefix = "> "
			}
			shared.PrintAt(screen, r.X+2, r.Y+5+index-start, shared.Truncate(prefix+nodeLabel(node, frame.NerdFont), r.W-4), nodeColor(node, colors))
		}
	}
	help := storeFormHelp(r.W - 4)
	shared.PrintAt(screen, r.X+2, r.Y+r.H-2, help, colors.Muted)
}

func storeFormHelp(width int) string {
	return shared.FitHints(width, "↑/↓ select  •  <tab> folder", "↑/↓ select  •  <tab>", "<tab> folder")
}

func (s *StoreForm) storeFormRect(viewport shared.Rect) (shared.Rect, int) {
	w := min(52, viewport.W-4)
	maxHeight := viewport.H - 4
	if s.choosingType {
		h := min(10, maxHeight)
		return shared.Rect{X: viewport.X + (viewport.W-w)/2, Y: viewport.Y + (viewport.H-h)/2, W: w, H: h}, 0
	}
	if s.editingValue {
		h := min(11, maxHeight)
		return shared.Rect{X: viewport.X + (viewport.W-w)/2, Y: viewport.Y + (viewport.H-h)/2, W: w, H: h}, 0
	}
	visible := min(5, min(max(0, maxHeight-8), len(s.storeNodes)))
	return shared.Rect{X: viewport.X + (viewport.W-w)/2, Y: viewport.Y + max(2, (viewport.H-(8+visible))/2), W: w, H: 8 + visible}, visible
}
