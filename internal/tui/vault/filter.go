package vault

import (
	"strings"

	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

func (s *View) filteredVaultNodes() []vault.Node {
	if !s.vaultFiltering {
		return s.vaultNodes
	}
	source := s.vaultNodes
	if s.searchLoaded {
		source = s.searchNodes
	}
	needle := strings.ToLower(s.vaultFilter)
	nodes := make([]vault.Node, 0, len(source))
	for _, node := range source {
		if strings.Contains(strings.ToLower(s.searchName(node)), needle) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}
func (s *View) handleVaultFilterInput(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		s.CancelSearch()
		s.vaultFiltering, s.vaultFilter, s.vaultSelected = false, "", 0
		s.searchNodes = nil
		s.searchLoaded = false
	case tcell.KeyCtrlU:
		s.vaultFilter, s.vaultSelected = "", 0
	case tcell.KeyCtrlW:
		s.vaultFilter, s.vaultSelected = shared.DeletePreviousWord(s.vaultFilter), 0
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		s.vaultFilter, s.vaultSelected = shared.DeleteLastRune(s.vaultFilter), 0
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

func (s *View) searchName(node vault.Node) string {
	if node.Shared {
		return node.Name
	}
	prefix := s.vaultPath.String()
	if prefix != "" {
		prefix += "/"
	}
	name := strings.TrimPrefix(node.Path.String(), prefix)
	if name == "" {
		return node.Name
	}
	return name
}
