package vault

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/vault"
)

func (s *View) HandleKey(event *tcell.EventKey) {
	if s.IsConfirmingDelete() {
		s.handleDeleteKey(event)
		return
	}
	if s.vaultFiltering {
		s.handleVaultFilterInput(event)
		return
	}
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
		case 'p':
			if s.deps.OpenPicker != nil {
				s.deps.OpenPicker()
			}
		case 'e':
			node, ok := s.selectedVaultNode()
			if ok && node.Kind == vault.EntryNode && s.deps.OpenEdit != nil {
				s.deps.OpenEdit(node.Path)
			}
		case 'd':
			s.openDelete()
		case 'a':
			if s.deps.OpenAdd != nil {
				s.deps.OpenAdd()
			}
		case '/':
			s.vaultFiltering, s.vaultFilter, s.vaultSelected = true, "", 0
			s.loadSearchNodes()
		case 'j':
			s.moveVaultSelection(1)
		case 'k':
			s.moveVaultSelection(-1)
		case 'h':
			s.vaultParent()
		case 'l':
			s.openVaultNode()
		case 't':
			if s.deps.OpenThemes != nil {
				s.deps.OpenThemes()
			}
		}
	}
}
