package tui

import "lazypass/internal/tui/shared"

const footerMaxRows = 2

func (s *screen) footerActions() shared.Footer {
	if s.route == VaultRoute {
		if s.vault.IsConfirmingDelete() {
			return shared.Footer{}
		}
		return s.vault.FooterActions()
	}
	return s.generator.FooterActions()
}

func (s *screen) footerRows(width int) []string {
	if s.route == VaultRoute && s.vault.IsConfirmingDelete() {
		return s.vault.FooterRows(width, footerMaxRows)
	}
	return s.footerActions().Rows(max(0, width-2), shared.FooterOptions{MaxRows: footerMaxRows})
}
