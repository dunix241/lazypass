package tui

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/tui/shared"
)

const footerMaxRows = 2

func (s *screen) footerActions() shared.Footer {
	if s.cfg.Compact {
		return shared.Footer{Actions: []shared.Action{{Hint: "?", Label: "keybindings", Key: tcell.NewEventKey(tcell.KeyRune, '?', 0)}}}
	}
	return s.viewActions()
}

func (s *screen) viewActions() shared.Footer {
	if s.route == VaultRoute {
		if s.vault.IsConfirmingDelete() {
			return shared.Footer{}
		}
		return s.vault.FooterActions()
	}
	return s.generator.FooterActions()
}

func (s *screen) footerRows(width int) []string {
	if !s.cfg.Compact && s.route == VaultRoute && s.vault.IsConfirmingDelete() {
		return s.vault.FooterRows(width, footerMaxRows)
	}
	return s.footerActions().Rows(max(0, width-2), shared.FooterOptions{MaxRows: footerMaxRows})
}
