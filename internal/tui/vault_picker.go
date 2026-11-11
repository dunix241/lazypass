package tui

import (
	"context"
	"encoding/json"
	"lazypass/internal/tui/shared"
	vaultui "lazypass/internal/tui/vault"
	"lazypass/internal/vault"
	"lazypass/internal/vault/browsercache"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (s *screen) activateVault(index int) {
	s.vaultActivation++
	if s.vault != nil {
		s.vault.CancelMarkedDelete()
		s.vault.CancelTransfer()
		s.vault.ClearMarks()
		s.vault.ClearAuthenticators()
		s.vault.CancelCopy()
		s.vault.CancelSearch()
	}
	s.vaultIndex = index
	selected := s.vaults[index]
	notify := func(level shared.Level, message string) {
		if s.vaultIndex == index {
			s.notify(level, message)
		}
	}
	progress := func(message string) {
		if s.vaultIndex == index {
			s.notifyWithDuration(shared.Info, message, -1)
		}
	}
	if s.vaultViews[index] == nil {
		identity, _ := json.Marshal(struct {
			ConfigPath, Name string
			Vaults           any
		}{s.cfgPath, selected.Name, s.cfg.Vaults})
		s.vaultViews[index] = vaultui.New(vaultui.Dependencies{
			Progress: progress,
			Cache:    browsercache.New(string(identity)),
			Context: func() context.Context {
				if s.operationContext != nil {
					return s.operationContext
				}
				return nil
			},
			Name:         selected.Name,
			Service:      selected.Service,
			RunPrompted:  s.runPrompted,
			RunOperation: s.operationRunner(selected.Service.Provider),
			Notify:       notify,
			NotifyRecovery: func(message, command string) {
				if s.vaultIndex == index {
					s.notifyRecovery(message, command)
				}
			},
			OpenThemes: s.openThemePanel,
			OpenAdd:    s.openVaultAdd,
			OpenEdit:   s.openVaultEdit,
			Dispatch:   s.dispatch,
		})
	}
	s.vault = s.vaultViews[index]
	s.store = vaultui.NewStoreForm(vaultui.StoreDependencies{
		Progress:      progress,
		Reconcile:     s.vault.Reconcile,
		LocationLabel: s.vault.LocationLabel,
		Service:       selected.Service,
		Password:      s.generator.Password,
		Notify:        notify,
		Closed:        func() { s.overlay = noOverlay },
		RunPrompted:   s.runPrompted,
		RunOperation:  s.operationRunner(selected.Service.Provider),
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
	s.vaultPickerViewport = shared.ListViewport{}
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
		s.refreshVaultEarly()
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
	r := shared.CenteredPanel(v, w, h)
	shared.Panel{Bounds: r, Title: "Vaults", TitleColor: frame.Palette.Accent, Footer: "↑/↓ select", FooterColor: frame.Palette.Muted, Border: frame.Palette.Border, Background: frame.Palette.Surface}.Draw(screen, func(content shared.Rect) {
		start, _ := s.vaultPickerViewport.Window(len(s.vaults), h-4, s.vaultPickerSelected)
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
			shared.PrintAt(screen, content.X, content.Y+1+row, shared.Truncate(label, content.W), color)
		}
	})
}

func (s *screen) moveVaultPicker(delta int) {
	if len(s.vaults) > 0 {
		s.vaultPickerSelected = (s.vaultPickerSelected + delta + len(s.vaults)) % len(s.vaults)
	}
}

func (s *screen) handleVaultPickerMouse(action tview.MouseAction, event *tcell.EventMouse) {
	v := s.frame().Viewport
	r := shared.CenteredPanel(v, min(60, v.W-2), min(len(s.vaults)+4, v.H-2))
	x, y := event.Position()
	if !r.Contains(x, y) {
		return
	}
	switch action {
	case tview.MouseScrollUp:
		s.moveVaultPicker(-1)
	case tview.MouseScrollDown:
		s.moveVaultPicker(1)
	case tview.MouseLeftClick, tview.MouseLeftDoubleClick:
		start, end := s.vaultPickerViewport.Window(len(s.vaults), r.H-4, s.vaultPickerSelected)
		index := start + y - r.Y - 2
		if x < r.X+2 || x >= r.X+r.W-2 || y < r.Y+2 || index < start || index >= end {
			return
		}
		s.vaultPickerSelected = index
		if action == tview.MouseLeftDoubleClick {
			s.handleVaultPicker(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
		}
	}
}

func (s *screen) closeVaults() {
	s.finishUnlock(unlockReply{err: context.Canceled})
	for _, view := range s.vaultViews {
		if view != nil {
			view.CancelMarkedDelete()
			view.CancelTransfer()
			view.ClearMarks()
			view.ClearAuthenticators()
			view.CancelCopy()
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
