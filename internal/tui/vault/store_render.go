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
	if s.editEntry != nil {
		title = " Edit entry "
	}
	shared.PrintAt(screen, r.X+2, r.Y, title, colors.Accent)
	if s.editingValue {
		shared.PrintAt(screen, r.X+2, r.Y+2, shared.Truncate("Entry: "+s.storePath, r.W-4), colors.Muted)
		shared.PrintAt(screen, r.X+2, r.Y+3, "Value (hidden):", colors.Text)
		shared.PrintAt(screen, r.X+2, r.Y+4, strings.Repeat("*", min(utf8.RuneCountInString(s.value), max(0, r.W-5)))+"_", colors.Focus)
		if s.editEntry == nil {
			shared.PrintPanelFooter(screen, r, "<shift-tab> path", colors.Muted)
		}
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
	visible := min(5, min(max(0, maxHeight-8), len(s.storeNodes)))
	return shared.Rect{X: viewport.X + (viewport.W-w)/2, Y: viewport.Y + max(2, (viewport.H-(8+visible))/2), W: w, H: 8 + visible}, visible
}
