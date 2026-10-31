package vault

import (
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

func (s *StoreForm) Draw(screen tcell.Screen, frame shared.Frame) {
	r, visible := s.storeFormRect(frame.Viewport)
	colors := frame.Palette
	shared.DrawBox(screen, r, colors.Focus, colors.Surface)
	shared.PrintAt(screen, r.X+2, r.Y, " Store generated password ", colors.Accent)
	shared.PrintAt(screen, r.X+2, r.Y+2, "Destination (folder/name):", colors.Muted)
	shared.PrintAt(screen, r.X+2, r.Y+3, shared.Truncate(s.storePath+"_", r.W-4), colors.Text)
	if s.storeMessage != "" {
		shared.PrintAt(screen, r.X+2, r.Y+5, shared.Truncate(s.storeMessage, r.W-4), colors.Warning)
	} else if visible > 0 {
		shared.PrintAt(screen, r.X+2, r.Y+5, "Folders (Tab inserts selected):", colors.Muted)
		for i := 0; i < visible; i++ {
			prefix := "  "
			if i == s.storeSelected {
				prefix = "> "
			}
			shared.PrintAt(screen, r.X+2, r.Y+6+i, shared.Truncate(prefix+nodeLabel(s.storeNodes[i], frame.NerdFont), r.W-4), colors.Accent)
		}
	}
	shared.PrintAt(screen, r.X+2, r.Y+r.H-2, storeFormHelp(r.W-4), colors.Muted)
}

func storeFormHelp(width int) string {
	for _, hint := range []string{"↑/↓ select  •  Tab folder  •  Enter save  •  Esc cancel", "↑/↓ select  •  Tab  •  Enter save  •  Esc cancel", "Tab pick  •  Enter save  •  Esc cancel", "Enter save  •  Esc cancel"} {
		if uniseg.StringWidth(hint) <= width {
			return hint
		}
	}
	return "Esc cancel"
}

func (s *StoreForm) storeFormRect(viewport shared.Rect) (shared.Rect, int) {
	w := min(52, viewport.W-4)
	maxHeight := viewport.H - 4
	visible := min(4, min(max(0, maxHeight-9), len(s.storeNodes)))
	return shared.Rect{X: viewport.X + (viewport.W-w)/2, Y: viewport.Y + max(2, (viewport.H-(9+visible))/2), W: w, H: 9 + visible}, visible
}
