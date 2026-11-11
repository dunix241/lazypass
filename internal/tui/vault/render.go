package vault

import (
	"fmt"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

func (s *View) Draw(screen tcell.Screen, frame shared.Frame) {
	s.authenticators.card = frame.Card
	defer s.drawDelete(screen, frame)
	r, colors := frame.Card, frame.Palette
	path := s.LocationLabel(s.vaultPath)
	title := "Vault"
	if s.deps.Name != "" {
		title += " · " + s.deps.Name
	}
	if len(s.marks) > 0 {
		title += fmt.Sprintf(" · %d marked", len(s.marks))
	}
	shared.Panel{Bounds: r, Title: title + ": " + path, TitleColor: colors.Accent, Border: colors.Border, Background: colors.Surface}.Draw(screen, nil)
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
	chip, code, remaining := s.authenticatorChip(node, r, time.Now())
	labelWidth := r.W - 4
	if code != "" {
		labelWidth = chip.X - r.X - 3
	}
	background := frame.Palette.Surface
	if s.marks[markID(node)] {
		background = frame.Palette.Border
		style := tcell.StyleDefault.Background(background)
		for x := r.X + 2; x < r.X+r.W-2; x++ {
			screen.SetContent(x, y, ' ', nil, style)
		}
	}
	style := tcell.StyleDefault.Foreground(nodeColor(node, frame.Palette)).Background(background)
	shared.PrintStyled(screen, r.X+2, y, shared.Truncate("  "+nodeLabel(node, frame.NerdFont), labelWidth), style)
	if code == "Loading…" {
		shared.PrintStyled(screen, chip.X, y, code, tcell.StyleDefault.Foreground(frame.Palette.Muted).Background(background))
	} else if code != "" {
		chipBackground := frame.Palette.Border
		if s.marks[markID(node)] {
			chipBackground = frame.Palette.Base
		}
		style := tcell.StyleDefault.Foreground(frame.Palette.Text).Background(chipBackground)
		for offset, ch := range " " + code + " " {
			screen.SetContent(chip.X+offset, y, ch, nil, style)
		}
		if chip.X+chip.W < r.X+r.W-2 {
			color := frame.Palette.Muted
			if remaining <= 5 {
				color = frame.Palette.Warning
			}
			shared.PrintStyled(screen, chip.X+chip.W+1, y, fmt.Sprintf("%5ds", remaining), tcell.StyleDefault.Foreground(color).Background(background))
		}
	}
	if selected && s.deleteTarget == nil {
		screen.ShowCursor(r.X+2, y)
	}
}

func (s *View) LocationLabel(path vault.Path) string {
	labels := make([]string, len(path))
	for i, part := range path {
		labels[i] = part
		if label := s.locationLabels[path[:i+1].String()]; label != "" {
			labels[i] = label
		} else if len(part) == 36 {
			labels[i] = part[:8]
		}
	}
	return "/" + strings.Join(labels, "/")
}

func (s *View) vaultListWindow(card shared.Rect) (listY, limit, start int) {
	listY, limit = card.Y+2, card.H-3
	if s.vaultFiltering {
		listY, limit = listY+1, limit-1
	}
	start, _ = s.viewport.Window(len(s.vaultDisplayNodes()), limit, s.vaultSelected)
	return listY, limit, start
}
