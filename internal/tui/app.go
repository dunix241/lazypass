package tui

import (
	"context"
	"fmt"
	"time"

	"lazypass/internal/config"
	"lazypass/internal/debug"
	"lazypass/internal/generator"
	"lazypass/internal/theme"
	generatorui "lazypass/internal/tui/generator"
	"lazypass/internal/tui/shared"
	"lazypass/internal/tui/themepanel"
	vaultui "lazypass/internal/tui/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Route uint8

const (
	GeneratorRoute Route = iota
	VaultRoute
)

type Option func(*screen)

func WithVault(v service.Service) Option  { return func(s *screen) { s.vaultService = v } }
func WithInitialRoute(route Route) Option { return func(s *screen) { s.route = route } }

type NamedVault struct {
	Name    string
	Service service.Service
}

func WithVaults(vaults []NamedVault) Option { return func(s *screen) { s.vaults = vaults } }

type App struct {
	app    *tview.Application
	screen *screen
}

func NewApp(cfg config.Config, cfgPath string, onCopy func(string) error, options ...Option) *App {
	s := &screen{Box: tview.NewBox(), cfg: cfg, cfgPath: cfgPath, colors: shared.DefaultPalette()}
	if palette, err := theme.Load(cfg.Theme); err == nil {
		if loaded, err := shared.NewPalette(palette); err == nil {
			s.colors = loaded
		} else {
			debug.Failure("theme palette", debug.Invalid)
		}
	} else {
		debug.Failure("theme load", debug.Unavailable)
		s.notify(shared.Warning, fmt.Sprintf("Theme %q unavailable; using %s", cfg.Theme, theme.DefaultName()))
	}
	for _, option := range options {
		option(s)
	}
	app := tview.NewApplication().EnableMouse(true).EnablePaste(true)
	s.quit, s.suspend = app.Stop, app.Suspend
	s.dispatch = func(ctx context.Context, fn func()) {
		if ctx.Err() == nil {
			app.QueueUpdateDraw(func() {
				if ctx.Err() == nil {
					fn()
				}
			})
		}
	}
	s.generator = generatorui.New(cfg.ToOptions(), generatorui.Dependencies{
		Copy:        onCopy,
		SaveOptions: s.saveOptions,
		Notify:      s.notify,
		OpenStore:   s.openStoreForm,
		OpenThemes:  s.openThemePanel,
	})
	if len(s.vaults) == 0 {
		s.vaults = []NamedVault{{Service: s.vaultService}}
	}
	s.vaultViews = make([]*vaultui.View, len(s.vaults))
	initialVault := 0
	for i, item := range s.vaults {
		if item.Name == cfg.LastVault {
			initialVault = i
			break
		}
	}
	s.activateVault(initialVault)
	s.themePanel = themepanel.New(themepanel.Dependencies{
		CurrentPalette:  func() shared.Palette { return s.colors },
		CurrentTheme:    func() string { return s.cfg.Theme },
		SetPalette:      func(p shared.Palette) { s.colors = p },
		SaveTheme:       s.saveTheme,
		UseDefaultTheme: s.useDefaultTheme,
		Notify:          s.notify,
		Closed:          func() { s.overlay = noOverlay },
	})
	return &App{app: app, screen: s}
}

func (a *App) CurrentPassword() string           { return a.screen.generator.Password() }
func (a *App) CurrentOptions() generator.Options { return a.screen.generator.Options() }
func (a *App) CurrentRoute() Route               { return a.screen.route }

func (a *App) Init() error {
	if err := a.screen.generator.Regenerate(); err != nil {
		return err
	}
	a.app.SetRoot(a.screen, true).SetFocus(a.screen)
	return nil
}
func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	a.screen.operationContext = ctx
	a.screen.running = true
	initialDraw := true
	a.app.SetBeforeDrawFunc(func(tcell.Screen) bool {
		if initialDraw {
			initialDraw = false
			if a.screen.route == VaultRoute {
				a.screen.vault.Refresh()
			}
		}
		return false
	})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.screen.dispatch(ctx, func() { a.screen.vault.RefreshMetadata(ctx) })
			}
		}
	}()
	defer func() { cancel(); a.screen.closeVaults() }()
	return a.app.Run()
}
