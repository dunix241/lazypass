package tui

import (
	"context"
	"lazypass/internal/tui/shared"
	vaultui "lazypass/internal/tui/vault"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

func (s *screen) activateVault(index int) {
	if s.vault != nil {
		s.vault.CancelSearch()
	}
	s.vaultIndex = index
	selected := s.vaults[index]
	notify := func(level shared.Level, message string) {
		if s.vaultIndex == index {
			s.notify(level, message)
		}
	}
	if s.vaultViews[index] == nil {
		s.vaultViews[index] = vaultui.New(vaultui.Dependencies{
			Name:         selected.Name,
			Service:      selected.Service,
			RunPrompted:  s.runPrompted,
			RunOperation: s.operationRunner(selected.Service.Provider),
			Notify:       notify,
			OpenThemes:   s.openThemePanel,
			OpenAdd:      s.openVaultAdd,
			OpenEdit:     s.openVaultEdit,
			Dispatch:     s.dispatch,
		})
	}
	s.vault = s.vaultViews[index]
	s.store = vaultui.NewStoreForm(vaultui.StoreDependencies{
		Service:      selected.Service,
		Password:     s.generator.Password,
		Notify:       notify,
		Closed:       func() { s.overlay = noOverlay },
		RunPrompted:  s.runPrompted,
		RunOperation: s.operationRunner(selected.Service.Provider),
		Saved: func(vault.Path) {
			if s.route == VaultRoute {
				s.vault.Refresh()
			}
		},
	})
}

func (s *screen) openVaultPicker() {
	if len(s.vaults) == 0 {
		s.notify(shared.Warning, "No vaults configured")
		return
	}
	s.vaultPickerSelected = s.vaultIndex
	s.openOverlay(vaultPickerOverlay, func() bool { return true })
}

func (s *screen) handleVaultPicker(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		s.overlay = noOverlay
	case tcell.KeyUp, tcell.KeyBacktab:
		s.moveVaultPicker(-1)
	case tcell.KeyDown, tcell.KeyTAB:
		s.moveVaultPicker(1)
	case tcell.KeyEnter:
		s.activateVault(s.vaultPickerSelected)
		if err := s.saveVaultSelection(); err != nil {
			s.notify(shared.Warning, "Unable to save selected vault")
		}
		s.vault.Refresh()
		s.overlay = noOverlay
	case tcell.KeyRune:
		switch event.Rune() {
		case 'q':
			s.overlay = noOverlay
		case 'j':
			s.moveVaultPicker(1)
		case 'k':
			s.moveVaultPicker(-1)
		}
	}
}

func (s *screen) drawVaultPicker(screen tcell.Screen, frame shared.Frame) {
	v := frame.Viewport
	w, h := min(60, v.W-2), min(len(s.vaults)+4, v.H-2)
	r := shared.Rect{X: v.X + (v.W-w)/2, Y: v.Y + (v.H-h)/2, W: w, H: h}
	shared.DrawBox(screen, r, frame.Palette.Border, frame.Palette.Surface)
	shared.PrintAt(screen, r.X+2, r.Y, " Vaults ", frame.Palette.Accent)
	start := max(0, s.vaultPickerSelected-(h-4)+1)
	for row := 0; row < h-4 && start+row < len(s.vaults); row++ {
		i := start + row
		prefix, color := "  ", frame.Palette.Text
		if i == s.vaultPickerSelected {
			prefix, color = "> ", frame.Palette.Focus
		}
		item := s.vaults[i]
		label := prefix + item.Name
		if item.Service.Provider != nil {
			label += " (" + item.Service.Provider.Name() + ")"
		}
		shared.PrintAt(screen, r.X+2, r.Y+2+row, shared.Truncate(label, w-4), color)
	}
	shared.PrintPanelFooter(screen, r, "↑/↓ select", frame.Palette.Muted)
}

func (s *screen) moveVaultPicker(delta int) {
	if len(s.vaults) > 0 {
		s.vaultPickerSelected = (s.vaultPickerSelected + delta + len(s.vaults)) % len(s.vaults)
	}
}

func (s *screen) closeVaults() {
	s.finishUnlock(unlockReply{err: context.Canceled})
	for _, view := range s.vaultViews {
		if view != nil {
			view.CancelSearch()
		}
	}
	for _, item := range s.vaults {
		if locker, ok := item.Service.Provider.(vault.Locker); ok {
			locker.Lock()
		}
	}
	if s.store != nil {
		s.store.Close()
	}
}
