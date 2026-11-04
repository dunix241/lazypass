package vault

import (
	"lazypass/internal/tui/shared"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

func (s *StoreForm) Draw(screen tcell.Screen, frame shared.Frame) {
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
		rows := min(len(fields), max(1, r.H-6))
		start := max(0, s.field-rows+1)
		for row := 0; row < rows; row++ {
			i := start + row
			field := fields[i]
			color := colors.Text
			value := strings.ReplaceAll(s.displayField(i), "\n", " ↵ ")
			if field.secret {
				value = strings.Repeat("*", min(utf8.RuneCountInString(value), max(0, r.W-18)))
			}
			if i == s.field {
				color = colors.Focus
				value += "_"
			}
			shared.PrintAt(screen, r.X+2, r.Y+4+row, shared.Truncate(field.label+": "+value, r.W-4), color)
		}
		hint := "<tab> field"
		if fields[s.field].multiline {
			hint += "  •  <ctrl-n> newline"
		}
		shared.PrintPanelFooter(screen, r, shared.FitHints(r.W-4, hint, "<tab> field"), colors.Muted)
		return
	}
	shared.PrintAt(screen, r.X+2, r.Y+2, "Destination (folder/name):", colors.Muted)
	shared.PrintAt(screen, r.X+2, r.Y+3, shared.Truncate(s.storePath+"_", r.W-4), colors.Text)
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
