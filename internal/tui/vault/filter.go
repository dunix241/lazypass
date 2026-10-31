package vault

import (
	"strings"

	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

func (s *View) filteredVaultNodes() []vault.Node {
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
func (s *View) handleVaultFilterInput(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		s.vaultFiltering, s.vaultFilter, s.vaultSelected = false, "", 0
	case tcell.KeyCtrlU:
		s.vaultFilter, s.vaultSelected = "", 0
	case tcell.KeyCtrlW:
		s.vaultFilter, s.vaultSelected = shared.DeletePreviousWord(s.vaultFilter), 0
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
