package vault

import (
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"

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
	title := "Vault"
	if s.deps.Name != "" {
		title += " · " + s.deps.Name
	}
	switch {
	case s.deleting:
		title += " · deleting…"
	case s.copying:
		title += " · copying…"
	case s.refreshing:
		title += " · refreshing…"
	}
	shared.PrintAt(screen, r.X+2, r.Y, shared.Truncate(" "+title+": "+path+" ", r.W-4), colors.Accent)
	if s.vault.Provider == nil {
		shared.PrintAt(screen, r.X+2, r.Y+2, shared.Truncate("Vault unavailable. Configure vault.provider.", r.W-4), colors.Warning)
		return
	}
	if s.loading {
		shared.PrintAt(screen, r.X+2, r.Y+2, "Loading…", colors.Muted)
		return
	}
	if s.vaultFiltering {
		label := "Search: " + s.vaultFilter + "_"
		if s.searching {
			label += " (searching…)"
		}
		shared.PrintAt(screen, r.X+2, r.Y+1, shared.Truncate(label, r.W-4), colors.Muted)
	}
	if s.vaultError != "" {
		listY, _, _ := s.vaultListWindow(r)
		if s.showVaultParentRow() {
			parent := s.vaultParentNode()
			s.drawBrowserRow(screen, frame, r, parent, listY, s.vaultSelected == 0)
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
		node := nodes[i]
		if s.vaultFiltering {
			node.Name = s.searchName(node)
		}
		s.drawBrowserRow(screen, frame, r, node, listY+i-start, i == s.vaultSelected)
	}
}

func (s *View) drawBrowserRow(screen tcell.Screen, frame shared.Frame, r shared.Rect, node vault.Node, y int, selected bool) {
	shared.PrintAt(screen, r.X+2, y, shared.Truncate("  "+nodeLabel(node, frame.NerdFont), r.W-4), nodeColor(node, frame.Palette))
	if selected && s.deleteTarget == nil {
		screen.ShowCursor(r.X+2, y)
	}
}

func (s *View) vaultListWindow(card shared.Rect) (listY, limit, start int) {
	listY, limit = card.Y+2, card.H-3
	if s.vaultFiltering {
		listY, limit = listY+1, limit-1
	}
	return listY, limit, max(0, s.vaultSelected-limit+1)
}
