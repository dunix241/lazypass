package vault

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/vault"
)

func (s *View) HandleKey(event *tcell.EventKey) {
	if s.IsTransferActive() {
		s.handleTransferKey(event)
		return
	}
	if s.loading {
		return
	}
	if s.IsConfirmingDelete() {
		s.handleDeleteKey(event)
		return
	}
	if s.vaultFiltering {
		s.handleVaultFilterInput(event)
		return
	}
	switch event.Key() {
	case tcell.KeyEscape:
		s.marks = nil
		s.ClearStagedTransfers()
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
		case 'c':
			s.copyEntry(false)
		case 'y':
			s.stageTransfer(false)
		case 'x':
			s.stageTransfer(true)
		case 'p':
			s.pasteEntries()
		case 'm':
			s.toggleMark(false)
		case 'M':
			s.toggleMark(true)
		case 'C':
			s.copyEntry(true)
		case 'r':
			s.openRename()
		case 'R':
			s.refreshProvider()
		case 'e':
			node, ok := s.selectedVaultNode()
			if ok && node.Kind == vault.EntryNode && s.deps.OpenEdit != nil {
				s.deps.OpenEdit(node.Path)
			}
		case 'd':
			s.deleteRecursive = false
			s.openDelete()
		case 'D':
			s.deleteRecursive = true
			s.openDelete()
		case 'a':
			if s.deps.OpenAdd != nil {
				s.deps.OpenAdd()
			}
		case '/':
			s.marks = nil
			s.ClearAuthenticators()
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
