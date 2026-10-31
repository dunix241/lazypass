package vault

import (
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

func (s *View) Draw(screen tcell.Screen, frame shared.Frame) {
	defer s.drawDelete(screen, frame)
	r, colors := frame.Card, frame.Palette
	shared.DrawBox(screen, r, colors.Border, colors.Surface)
	path := "/"
	if len(s.vaultPath) > 0 {
		path += s.vaultPath.String()
	}
	shared.PrintAt(screen, r.X+2, r.Y, " Vault: "+path+" ", colors.Accent)
	if s.vault.Provider == nil {
		shared.PrintAt(screen, r.X+2, r.Y+2, "Vault unavailable. Configure vault.provider: pass.", colors.Warning)
		return
	}
	if s.vaultFiltering {
		shared.PrintAt(screen, r.X+2, r.Y+1, shared.Truncate("Search: "+s.vaultFilter+"_", r.W-4), colors.Muted)
	}
	if s.vaultError != "" {
		listY, _, _ := s.vaultListWindow(r)
		if s.showVaultParentRow() {
			parent := s.vaultParentNode()
			prefix := "  "
			if s.vaultSelected == 0 {
				prefix = "> "
			}
			shared.PrintAt(screen, r.X+2, listY, shared.Truncate(prefix+nodeLabel(parent, frame.NerdFont), r.W-4), nodeColor(parent, colors))
			listY++
		}
		shared.PrintAt(screen, r.X+2, listY, shared.Truncate(s.vaultError, r.W-4), colors.Warning)
		return
	}
	nodes := s.vaultDisplayNodes()
	if len(nodes) == 0 {
		message := "No entries in this folder."
		messageY := r.Y + 2
		if s.vaultFiltering {
			message = "No matching folders or entries."
			messageY++
		}
		shared.PrintAt(screen, r.X+2, messageY, shared.Truncate(message, r.W-4), colors.Muted)
		return
	}
	listY, limit, start := s.vaultListWindow(r)
	for i := start; i < len(nodes) && i < start+limit; i++ {
		node, prefix := nodes[i], "  "
		if s.vaultFiltering {
			node.Name = s.searchName(node)
		}
		if i == s.vaultSelected {
			prefix = "> "
		}
		shared.PrintAt(screen, r.X+2, listY+i-start, shared.Truncate(prefix+nodeLabel(node, frame.NerdFont), r.W-4), nodeColor(node, colors))
	}
}

func (s *View) vaultListWindow(card shared.Rect) (listY, limit, start int) {
	listY, limit = card.Y+2, card.H-3
	if s.vaultFiltering {
		listY, limit = listY+1, limit-1
	}
	return listY, limit, max(0, s.vaultSelected-limit+1)
}
