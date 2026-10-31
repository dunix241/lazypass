package vault

import "github.com/gdamore/tcell/v2"

func (s *View) HandleKey(event *tcell.EventKey) {
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
			if s.deps.OpenThemes != nil {
				s.deps.OpenThemes()
			}
		}
	}
}
