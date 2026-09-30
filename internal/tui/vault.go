package tui

import (
	"context"
	"errors"
	"strings"

	"lazypass/internal/debug"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

func (s *screen) drawVault(screen tcell.Screen, l layout, ox, oy int) {
	r := l.card
	r.x += ox
	r.y += oy
	drawBox(screen, r, s.colors.border, s.colors.surface)
	path := "/"
	if len(s.vaultPath) > 0 {
		path += s.vaultPath.String()
	}
	printAt(screen, r.x+2, r.y, " Vault: "+path+" ", s.colors.accent)
	if s.vault.Provider == nil {
		printAt(screen, r.x+2, r.y+2, "Vault unavailable. Configure vault.provider: pass.", s.colors.warning)
		return
	}
	if s.vaultFiltering {
		printAt(screen, r.x+2, r.y+1, truncate("Filter: "+s.vaultFilter+"_", r.w-4), s.colors.muted)
	}
	if s.vaultError != "" {
		listY, _, _ := s.vaultListWindow(r)
		if s.showVaultParentRow() {
			parent := s.vaultParentNode()
			prefix := "  "
			if s.vaultSelected == 0 {
				prefix = "> "
			}
			printAt(screen, r.x+2, listY, truncate(prefix+s.vaultNodeLabel(parent), r.w-4), s.vaultNodeColor(parent))
			listY++
		}
		printAt(screen, r.x+2, listY, truncate(s.vaultError, r.w-4), s.colors.warning)
		return
	}
	nodes := s.vaultDisplayNodes()
	if len(nodes) == 0 {
		message := "No entries in this folder."
		if s.vaultFiltering {
			message = "No matching folders or entries."
		}
		messageY := r.y + 2
		if s.vaultFiltering {
			messageY++
		}
		printAt(screen, r.x+2, messageY, truncate(message, r.w-4), s.colors.muted)
		return
	}
	listY, limit, start := s.vaultListWindow(r)
	for i := start; i < len(nodes) && i < start+limit; i++ {
		node, prefix := nodes[i], "  "
		if i == s.vaultSelected {
			prefix = "> "
		}
		name, color := s.vaultNodeLabel(node), s.vaultNodeColor(node)
		rowY := listY + i - start
		printAt(screen, r.x+2, rowY, truncate(prefix+name, r.w-4), color)
	}
}

func (s *screen) vaultListWindow(card rect) (listY, limit, start int) {
	listY, limit = card.y+2, card.h-3
	if s.vaultFiltering {
		listY, limit = listY+1, limit-1
	}
	return listY, limit, max(0, s.vaultSelected-limit+1)
}

func (s *screen) filteredVaultNodes() []vault.Node {
	if !s.vaultFiltering || s.vaultFilter == "" {
		return s.vaultNodes
	}
	needle := strings.ToLower(s.vaultFilter)
	nodes := make([]vault.Node, 0, len(s.vaultNodes))
	for _, node := range s.vaultNodes {
		if strings.Contains(strings.ToLower(node.Name), needle) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (s *screen) showVaultParentRow() bool {
	return len(s.vaultPath) > 0 && !s.vaultFiltering
}

func (s *screen) vaultParentNode() vault.Node {
	parent := append(vault.Path(nil), s.vaultPath[:len(s.vaultPath)-1]...)
	return vault.Node{Name: "..", Path: parent, Kind: vault.FolderNode}
}

func (s *screen) isVaultParentNode(node vault.Node) bool {
	return node.Name == ".." && node.Kind == vault.FolderNode
}

func (s *screen) vaultDisplayNodes() []vault.Node {
	nodes := s.filteredVaultNodes()
	if !s.showVaultParentRow() {
		return nodes
	}
	display := make([]vault.Node, 0, len(nodes)+1)
	display = append(display, s.vaultParentNode())
	display = append(display, nodes...)
	return display
}

func (s *screen) vaultNodeLabel(node vault.Node) string {
	icon := ""
	if s.cfg.NerdFont {
		if node.Kind == vault.FolderNode {
			icon = " "
		} else {
			icon = "󰦝 "
		}
	}
	name := node.Name
	if node.Kind == vault.FolderNode {
		name += "/"
	}
	return icon + name
}

func (s *screen) vaultNodeColor(node vault.Node) tcell.Color {
	if node.Kind == vault.FolderNode {
		return s.colors.accent
	}
	return s.colors.text
}

func (s *screen) refreshVault() {
	s.vaultError = ""
	nodes, err := s.vault.List(context.Background(), s.vaultPath)
	if err != nil {
		if s.vault.Provider == nil {
			return
		}
		debug.Failure("vault list", vaultFailureCause(err))
		s.vaultError = "Vault unavailable or folder cannot be opened."
		nodes = nil
	}
	s.vaultNodes = nodes
	s.vaultSelected = min(s.vaultSelected, max(0, len(s.vaultDisplayNodes())-1))
}

func (s *screen) handleVaultInput(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyUp:
		s.moveVaultSelection(-1)
	case tcell.KeyDown:
		s.moveVaultSelection(1)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		s.vaultParent()
	case tcell.KeyEnter:
		s.openVaultNode()
	case tcell.KeyRune:
		switch event.Rune() {
		case '/':
			s.vaultFiltering, s.vaultFilter, s.vaultSelected = true, "", 0
		case 'j':
			s.moveVaultSelection(1)
		case 'k':
			s.moveVaultSelection(-1)
		case 'h':
			s.vaultParent()
		case 'l':
			s.openVaultNode()
		case 't':
			s.openThemePanel()
		}
	}
}

func (s *screen) handleVaultFilterInput(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		s.vaultFiltering, s.vaultFilter, s.vaultSelected = false, "", 0
	case tcell.KeyCtrlU:
		s.vaultFilter, s.vaultSelected = "", 0
	case tcell.KeyCtrlW:
		s.vaultFilter, s.vaultSelected = deletePreviousWord(s.vaultFilter), 0
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(s.vaultFilter) > 0 {
			runes := []rune(s.vaultFilter)
			s.vaultFilter, s.vaultSelected = string(runes[:len(runes)-1]), 0
		}
	case tcell.KeyUp:
		s.moveVaultSelection(-1)
	case tcell.KeyDown:
		s.moveVaultSelection(1)
	case tcell.KeyEnter:
		s.openVaultNode()
	case tcell.KeyRune:
		if event.Rune() >= ' ' {
			s.vaultFilter += string(event.Rune())
			s.vaultSelected = 0
		}
	}
}

func (s *screen) moveVaultSelection(delta int) {
	nodes := s.vaultDisplayNodes()
	s.vaultSelected = min(max(0, len(nodes)-1), max(0, s.vaultSelected+delta))
}

func (s *screen) vaultParent() {
	if len(s.vaultPath) == 0 {
		return
	}
	child := s.vaultPath
	s.vaultFiltering, s.vaultFilter = false, ""
	s.vaultPath = s.vaultPath[:len(s.vaultPath)-1]
	s.refreshVault()
	for i, node := range s.vaultDisplayNodes() {
		if !s.isVaultParentNode(node) && node.Path.String() == child.String() {
			s.vaultSelected = i
			return
		}
	}
}

func (s *screen) openVaultNode() {
	node, ok := s.selectedVaultNode()
	if !ok {
		return
	}
	if s.isVaultParentNode(node) {
		s.vaultParent()
		return
	}
	if node.Kind == vault.FolderNode {
		s.vaultPath, s.vaultSelected, s.vaultFiltering, s.vaultFilter = node.Path, 1, false, ""
		s.refreshVault()
		return
	}
	s.copyVaultEntry()
}

// runPrompted executes fn with the terminal released while the app is
// running, so subprocesses that need pinentry get a usable TTY. It falls
// back to a direct call when suspension is unavailable.
func (s *screen) runPrompted(fn func() error) error {
	if s.suspend == nil {
		return fn()
	}
	var err error
	if !s.suspend(func() { err = fn() }) {
		return fn()
	}
	return err
}

func (s *screen) copyVaultEntry() {
	node, ok := s.selectedVaultNode()
	if !ok || node.Kind != vault.EntryNode {
		return
	}
	if err := s.runPrompted(func() error { return s.vault.CopyPassword(context.Background(), node.Path) }); err != nil {
		var diagnostic vault.Diagnostic
		if !errors.As(err, &diagnostic) {
			debug.Failure("vault copy", vaultFailureCause(err))
		}
		s.notify(notificationError, copyErrorMessage(err))
		return
	}
	s.notify(notificationSuccess, "Copied to clipboard")
}

func (s *screen) selectedVaultNode() (vault.Node, bool) {
	nodes := s.vaultDisplayNodes()
	if s.vaultSelected < 0 || s.vaultSelected >= len(nodes) {
		return vault.Node{}, false
	}
	return nodes[s.vaultSelected], true
}
