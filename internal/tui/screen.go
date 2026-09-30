package tui

import (
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"lazypass/internal/config"
	"lazypass/internal/debug"
	"lazypass/internal/generator"
	"lazypass/internal/theme"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type focus int

const (
	focusLength focus = iota
	focusUpper
	focusLower
	focusNumbers
	focusSymbols
	focusAmbiguous
	focusStore
	focusRegen
	focusCopy
	focusCount
)

type Route uint8

const (
	GeneratorRoute Route = iota
	VaultRoute
)

type Option func(*screen)

func WithVault(v service.Service) Option  { return func(s *screen) { s.vault = v } }
func WithInitialRoute(route Route) Option { return func(s *screen) { s.route = route } }

// App owns application lifecycle; Screen is responsible for all viewport drawing.
type App struct {
	app    *tview.Application
	screen *screen
}

func NewApp(cfg config.Config, cfgPath string, onCopy func(string) error, options ...Option) *App {
	colors := defaultTUIPalette()
	s := &screen{Box: tview.NewBox(), cfg: cfg, cfgPath: cfgPath, onCopy: onCopy, selected: focusLength, colors: colors}
	if palette, err := theme.Load(cfg.Theme); err == nil {
		if loaded, err := newTUIPalette(palette); err == nil {
			s.colors = loaded
		} else {
			debug.Failure("theme palette", debug.Invalid)
		}
	} else {
		debug.Failure("theme load", debug.Unavailable)
		s.notify(notificationWarning, fmt.Sprintf("Theme %q unavailable; using %s", cfg.Theme, theme.DefaultName()))
	}
	s.lengthText = strconv.Itoa(cfg.Length)
	for _, option := range options {
		option(s)
	}
	app := tview.NewApplication().EnableMouse(true)
	s.quit = app.Stop
	s.suspend = app.Suspend
	return &App{app: app, screen: s}
}

func (a *App) CurrentPassword() string           { return a.screen.password }
func (a *App) CurrentOptions() generator.Options { return a.screen.cfg.ToOptions() }
func (a *App) CurrentRoute() Route               { return a.screen.route }

func (a *App) Init() error {
	if err := a.screen.regenerate(); err != nil {
		return err
	}
	if a.screen.route == VaultRoute {
		a.screen.refreshVault()
	}
	a.app.SetRoot(a.screen, true).SetFocus(a.screen)
	return nil
}

func (a *App) Run() error { return a.app.Run() }

type screen struct {
	*tview.Box
	cfg            config.Config
	cfgPath        string
	onCopy         func(string) error
	password       string
	bits           float64
	strength       string
	selected       focus
	lengthText     string
	status         string
	statusTill     time.Time
	statusLevel    notificationLevel
	quit           func()
	suspend        func(func()) bool
	draggingSlider bool
	colors         tuiPalette
	themePanel     themePanel
	route          Route
	vault          service.Service
	vaultPath      vault.Path
	vaultNodes     []vault.Node
	vaultSelected  int
	vaultError     string
	vaultFilter    string
	vaultFiltering bool
	storeForm      bool
	storePath      string
	storeNodes     []vault.Node
	storeSelected  int
	storeMessage   string
}

func (s *screen) save() {
	if s.cfgPath == "" {
		return
	}
	current, err := config.Load(s.cfgPath)
	if err != nil {
		return
	}
	s.cfg = current.WithOptions(s.cfg.ToOptions())
	_ = s.cfg.Save(s.cfgPath)
}

func (s *screen) saveTheme(name string) error {
	if s.cfgPath == "" {
		s.cfg.Theme = name
		return nil
	}
	current, err := config.Load(s.cfgPath)
	if err != nil {
		return err
	}
	current.Theme = name
	if err := current.Save(s.cfgPath); err != nil {
		return err
	}
	s.cfg = current
	return nil
}

func (s *screen) Draw(screen tcell.Screen) {
	x, y, width, height := s.GetRect()
	for yy := y; yy < y+height; yy++ {
		for xx := x; xx < x+width; xx++ {
			screen.SetContent(xx, yy, ' ', nil, tcell.StyleDefault.Background(s.colors.base))
		}
	}
	l := calculateLayout(width, height)
	if l.tooSmall {
		message := truncate("Resize terminal (min 32x18)", max(0, width-2))
		printAt(screen, x+max(1, (width-utf8.RuneCountInString(message))/2), y+height/2, message, s.colors.warning)
		return
	}
	logo := calculateLogoLayout(width, height)
	drawLogo(screen, logo, x, y, s.colors)
	s.drawHeaderStatus(screen, logo, x, y, width)
	if s.route == VaultRoute {
		s.drawVault(screen, l, x, y)
	} else {
		s.drawCard(screen, l, x, y)
	}
	footer := s.generatorFooter(width)
	if s.route == VaultRoute {
		footer = fitFooter(width, vaultFooter, vaultCompact, vaultShort, vaultMinimal)
	}
	printAt(screen, x+max(1, (width-utf8.RuneCountInString(footer))/2), y+height-1, footer, s.colors.muted)
	if s.themePanel.open {
		s.drawThemePanel(screen, x, y, width, height)
	}
	if s.storeForm {
		s.drawStoreForm(screen, x, y, width, height)
	}
}

func (s *screen) switchRoute() {
	if s.route == GeneratorRoute {
		s.route = VaultRoute
		s.refreshVault()
		return
	}
	s.route = GeneratorRoute
}
