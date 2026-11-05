package tui

import (
	"context"
	"time"

	"lazypass/internal/config"
	"lazypass/internal/task"
	generatorui "lazypass/internal/tui/generator"
	"lazypass/internal/tui/notification"
	"lazypass/internal/tui/shared"
	"lazypass/internal/tui/themepanel"
	vaultui "lazypass/internal/tui/vault"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

type overlay uint8

const (
	noOverlay overlay = iota
	themeOverlay
	storeOverlay
	vaultPickerOverlay
)

// screen is the single focused primitive and coordinates persistent child views.
type screen struct {
	notificationTimer    *time.Timer
	notificationDeadline time.Time
	drawnNotification    *notification.Message
	loggedNotification   *notification.Message
	notifications        notification.Manager
	notificationBounds   []shared.Rect
	unlock               *unlockPrompt
	running              bool
	operationContext     context.Context
	tasks                *task.Scheduler
	*tview.Box
	cfg                 config.Config
	cfgPath             string
	generator           *generatorui.View
	vault               *vaultui.View
	store               *vaultui.StoreForm
	themePanel          *themepanel.Panel
	route               Route
	overlay             overlay
	vaultService        service.Service
	colors              shared.Palette
	quit                func()
	suspend             func(func()) bool
	vaults              []NamedVault
	vaultViews          []*vaultui.View
	vaultIndex          int
	vaultPickerSelected int
	dispatch            func(context.Context, func())
}

func (s *screen) Draw(screen tcell.Screen) {
	screen.HideCursor()
	s.notificationBounds = nil
	s.drawnNotification = nil
	x, y, width, height := s.GetRect()
	for yy := y; yy < y+height; yy++ {
		for xx := x; xx < x+width; xx++ {
			screen.SetContent(xx, yy, ' ', nil, tcell.StyleDefault.Background(s.colors.Base))
		}
	}
	l := calculateLayout(width, height)
	if l.tooSmall {
		message := shared.Truncate("Resize terminal (min 32x18)", max(0, width-2))
		shared.PrintAt(screen, x+max(1, (width-uniseg.StringWidth(message))/2), y+height/2, message, s.colors.Warning)
		return
	}
	frame := s.frameFrom(l)
	logo := calculateLogoLayout(width, height)
	drawLogo(screen, logo, x, y, s.colors)
	s.drawHeaderStatus(screen, logo, x, y, width)
	footer := ""
	if s.route == VaultRoute {
		s.vault.Draw(screen, frame)
		footer = s.vault.Footer(width)
	} else {
		s.generator.Draw(screen, frame)
		footer = s.generator.Footer(width)
	}
	shared.PrintAt(screen, x+max(1, (width-uniseg.StringWidth(footer))/2), y+height-1, footer, s.colors.Muted)
	switch s.overlay {
	case themeOverlay:
		s.themePanel.Draw(screen, frame)
	case storeOverlay:
		s.store.Draw(screen, frame)
	case vaultPickerOverlay:
		s.drawVaultPicker(screen, frame)
	}
	if s.unlock != nil {
		s.drawUnlock(screen, frame)
	}
}

func (s *screen) switchRoute() {
	s.vault.CancelSearch()
	s.generator.ReleaseMouse()
	if s.route == GeneratorRoute {
		s.route = VaultRoute
		s.vault.Refresh()
		return
	}
	s.route = GeneratorRoute
}

func (s *screen) frame() shared.Frame {
	_, _, w, h := s.GetRect()
	return s.frameFrom(calculateLayout(w, h))
}

func (s *screen) frameFrom(l layout) shared.Frame {
	x, y, w, h := s.GetRect()
	return shared.Frame{
		Viewport: shared.Rect{X: x, Y: y, W: w, H: h},
		Card:     l.card.Translate(x, y),
		Compact:  l.compact,
		Short:    l.short,
		Palette:  s.colors,
		NerdFont: s.cfg.NerdFont,
	}
}

func (s *screen) openStoreForm() {
	s.openOverlay(storeOverlay, s.store.Open)
}

func (s *screen) openThemePanel() {
	s.openOverlay(themeOverlay, s.themePanel.Open)
}

func (s *screen) openVaultAdd() {
	s.openOverlay(storeOverlay, func() bool { return s.store.OpenAdd(s.vault.CurrentPath()) })
}

func (s *screen) openVaultEdit(path vault.Path) {
	s.openOverlay(storeOverlay, func() bool { return s.store.OpenEdit(path) })
}

func (s *screen) openOverlay(kind overlay, open func() bool) {
	if s.overlay != noOverlay {
		return
	}
	s.generator.ReleaseMouse()
	if open() {
		s.overlay = kind
	}
}
