package tui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"lazypass/internal/app"
	"lazypass/internal/config"
	"lazypass/internal/debug"
	"lazypass/internal/generator"
	"lazypass/internal/theme"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

type focus int

type notificationLevel uint8

const (
	notificationInfo notificationLevel = iota
	notificationSuccess
	notificationWarning
	notificationError
)

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

const (
	generatorFooterFull    = "<tab> focus  •  a add  •  r regenerate  •  c copy  •  t themes  •  v view  •  q quit"
	generatorFooterCompact = "a add  •  c copy  •  t themes  •  v view  •  q quit"
	generatorFooterShort   = "a add  •  v view  •  q quit"
	generatorFooterMinimal = "q quit"
	vaultFooter            = "h back  •  l open/copy  •  / filter  •  t themes  •  v view  •  q quit"
	vaultCompact           = "h back  •  l open/copy  •  / filter  •  v view  •  q quit"
	vaultShort             = "h back  •  l open/copy  •  v view  •  q quit"
	vaultMinimal           = "h back • l open/copy • q quit"
)

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

func (s *screen) regenerate() error {
	results, err := app.Generate(s.cfg.ToOptions(), 1)
	if err != nil {
		debug.Failure("generator regenerate", debug.Unknown)
		return err
	}
	s.password, s.bits, s.strength = results[0].Password, results[0].EntropyBits, results[0].Strength
	return nil
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

func (s *screen) notify(level notificationLevel, message string) {
	s.status, s.statusLevel, s.statusTill = message, level, time.Now().Add(2*time.Second)
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

func (s *screen) generatorFooter(width int) string {
	contextHint := ""
	switch s.selected {
	case focusLength:
		contextHint = "<left>/<right> length"
	case focusUpper, focusLower, focusNumbers, focusSymbols, focusAmbiguous:
		contextHint = "<space> toggle"
	}
	bases := []string{generatorFooterFull, generatorFooterCompact, generatorFooterShort, generatorFooterMinimal}
	if contextHint == "" {
		return fitFooter(width, bases...)
	}
	candidates := make([]string, 0, len(bases)+1)
	for _, base := range bases {
		candidates = append(candidates, contextHint+"  •  "+base)
	}
	candidates = append(candidates, contextHint)
	return fitFooter(width, candidates...)
}

func fitFooter(width int, options ...string) string {
	for _, option := range options {
		if utf8.RuneCountInString(option) <= width-2 {
			return option
		}
	}
	return options[len(options)-1]
}

func (s *screen) drawHeaderStatus(screen tcell.Screen, logo logoLayout, x, y, width int) {
	if s.status != "" && time.Now().Before(s.statusTill) {
		statusX := x + width - utf8.RuneCountInString(s.status) - 2
		if statusX > x+logo.right()+2 {
			printAt(screen, statusX, y, s.status, s.notificationColor(s.statusLevel))
		} else {
			rightWidth := width - logo.right() - 2
			leftWidth := logo.x - 2
			if rightWidth >= leftWidth {
				printAt(screen, x+logo.right()+1, y, truncate(s.status, rightWidth), s.notificationColor(s.statusLevel))
			} else {
				printAt(screen, x+1, y, truncate(s.status, leftWidth), s.notificationColor(s.statusLevel))
			}
		}
	}
}

func (s *screen) drawVault(screen tcell.Screen, l layout, ox, oy int) {
	r := l.card
	r.x += ox
	r.y += oy
	drawBox(screen, r, s.colors.border, s.colors.surface)
	path := "/"
	if len(s.vaultPath) > 0 {
		path += s.vaultPath.String()
	}
	printAt(screen, r.x+2, r.y, " Vault: "+path+" ", s.colors.accent)
	if s.vault.Provider == nil {
		printAt(screen, r.x+2, r.y+2, "Vault unavailable. Configure vault.provider: pass.", s.colors.warning)
		return
	}
	if s.vaultFiltering {
		printAt(screen, r.x+2, r.y+1, truncate("Filter: "+s.vaultFilter+"_", r.w-4), s.colors.muted)
	}
	if s.vaultError != "" {
		listY, _, _ := s.vaultListWindow(r)
		if s.showVaultParentRow() {
			parent := s.vaultParentNode()
			prefix := "  "
			if s.vaultSelected == 0 {
				prefix = "> "
			}
			printAt(screen, r.x+2, listY, truncate(prefix+s.vaultNodeLabel(parent), r.w-4), s.vaultNodeColor(parent))
			listY++
		}
		printAt(screen, r.x+2, listY, truncate(s.vaultError, r.w-4), s.colors.warning)
		return
	}
	nodes := s.vaultDisplayNodes()
	if len(nodes) == 0 {
		message := "No entries in this folder."
		if s.vaultFiltering {
			message = "No matching folders or entries."
		}
		messageY := r.y + 2
		if s.vaultFiltering {
			messageY++
		}
		printAt(screen, r.x+2, messageY, truncate(message, r.w-4), s.colors.muted)
		return
	}
	listY, limit, start := s.vaultListWindow(r)
	for i := start; i < len(nodes) && i < start+limit; i++ {
		node, prefix := nodes[i], "  "
		if i == s.vaultSelected {
			prefix = "> "
		}
		name, color := s.vaultNodeLabel(node), s.vaultNodeColor(node)
		rowY := listY + i - start
		printAt(screen, r.x+2, rowY, truncate(prefix+name, r.w-4), color)
	}
}

func (s *screen) vaultListWindow(card rect) (listY, limit, start int) {
	listY, limit = card.y+2, card.h-3
	if s.vaultFiltering {
		listY, limit = listY+1, limit-1
	}
	return listY, limit, max(0, s.vaultSelected-limit+1)
}

func (s *screen) filteredVaultNodes() []vault.Node {
	if !s.vaultFiltering || s.vaultFilter == "" {
		return s.vaultNodes
	}
	needle := strings.ToLower(s.vaultFilter)
	nodes := make([]vault.Node, 0, len(s.vaultNodes))
	for _, node := range s.vaultNodes {
		if strings.Contains(strings.ToLower(node.Name), needle) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (s *screen) showVaultParentRow() bool {
	return len(s.vaultPath) > 0 && !s.vaultFiltering
}

func (s *screen) vaultParentNode() vault.Node {
	parent := append(vault.Path(nil), s.vaultPath[:len(s.vaultPath)-1]...)
	return vault.Node{Name: "..", Path: parent, Kind: vault.FolderNode}
}

func (s *screen) isVaultParentNode(node vault.Node) bool {
	return node.Name == ".." && node.Kind == vault.FolderNode
}

func (s *screen) vaultDisplayNodes() []vault.Node {
	nodes := s.filteredVaultNodes()
	if !s.showVaultParentRow() {
		return nodes
	}
	display := make([]vault.Node, 0, len(nodes)+1)
	display = append(display, s.vaultParentNode())
	display = append(display, nodes...)
	return display
}

func (s *screen) notificationColor(level notificationLevel) tcell.Color {
	switch level {
	case notificationSuccess:
		return s.colors.accent
	case notificationWarning:
		return s.colors.warning
	case notificationError:
		return tcell.ColorRed
	default:
		return s.colors.focus
	}
}

func (s *screen) vaultNodeLabel(node vault.Node) string {
	icon := ""
	if s.cfg.NerdFont {
		if node.Kind == vault.FolderNode {
			icon = " "
		} else {
			icon = "󰦝 "
		}
	}
	name := node.Name
	if node.Kind == vault.FolderNode {
		name += "/"
	}
	return icon + name
}

func (s *screen) vaultNodeColor(node vault.Node) tcell.Color {
	if node.Kind == vault.FolderNode {
		return s.colors.accent
	}
	return s.colors.text
}

func (s *screen) drawStoreForm(screen tcell.Screen, x, y, width, height int) {
	r, visible := s.storeFormRect(x, y, width, height)
	drawBox(screen, r, s.colors.focus, s.colors.surface)
	printAt(screen, r.x+2, r.y, " Store generated password ", s.colors.accent)
	printAt(screen, r.x+2, r.y+2, "Destination (folder/name):", s.colors.muted)
	printAt(screen, r.x+2, r.y+3, truncate(s.storePath+"_", r.w-4), s.colors.text)
	if s.storeMessage != "" {
		printAt(screen, r.x+2, r.y+5, truncate(s.storeMessage, r.w-4), s.colors.warning)
	} else if visible > 0 {
		printAt(screen, r.x+2, r.y+5, "Folders (Tab inserts selected):", s.colors.muted)
		for i := 0; i < visible; i++ {
			prefix, color := "  ", s.colors.accent
			if i == s.storeSelected {
				prefix = "> "
			}
			rowY := r.y + 6 + i
			printAt(screen, r.x+2, rowY, truncate(prefix+s.vaultNodeLabel(s.storeNodes[i]), r.w-4), color)
		}
	}
	printAt(screen, r.x+2, r.y+r.h-2, storeFormHelp(r.w-4), s.colors.muted)
}

func storeFormHelp(width int) string {
	for _, hint := range []string{
		"↑/↓ select  •  Tab folder  •  Enter save  •  Esc cancel",
		"↑/↓ select  •  Tab  •  Enter save  •  Esc cancel",
		"Tab pick  •  Enter save  •  Esc cancel",
		"Enter save  •  Esc cancel",
	} {
		if uniseg.StringWidth(hint) <= width {
			return hint
		}
	}
	return "Esc cancel"
}

func (s *screen) storeFormRect(x, y, width, height int) (rect, int) {
	w := min(52, width-4)
	maxHeight := height - 4
	visible := min(4, min(max(0, maxHeight-9), len(s.storeNodes)))
	return rect{x + (width-w)/2, y + max(2, (height-(9+visible))/2), w, 9 + visible}, visible
}

func (s *screen) drawCard(screen tcell.Screen, l layout, ox, oy int) {
	r := l.card
	r.x += ox
	r.y += oy
	drawBox(screen, r, s.colors.border, s.colors.surface)
	printAt(screen, r.x+2, r.y, " Generate a password ", s.colors.accent)
	passwordLabelY, passwordY, meterY, lengthLabelY, charactersY := r.y+2, r.y+3, r.y+4, r.y+6, r.y+9
	if l.short && !l.compact {
		passwordLabelY, passwordY, meterY, lengthLabelY, charactersY = r.y+1, r.y+2, r.y+3, r.y+5, r.y+8
	}
	printAt(screen, r.x+2, passwordLabelY, "PASSWORD", s.colors.muted)
	pw := truncate(s.password, max(12, r.w-33))
	printAt(screen, r.x+2, passwordY, pw, s.colors.text)
	s.drawAction(screen, l.store, ox, oy, focusStore, "+ Add")
	if !l.compact {
		s.drawAction(screen, l.regen, ox, oy, focusRegen, "↻ Regenerate")
	}
	s.drawAction(screen, l.copy, ox, oy, focusCopy, "⧉ Copy")
	blocks := min(8, int(s.bits/16))
	meter := ""
	for i := 0; i < 8; i++ {
		if i < blocks {
			meter += "█"
		} else {
			meter += "░"
		}
	}
	printAt(screen, r.x+2, meterY, meter, s.colors.accent)
	strength := fmt.Sprintf("%d bits  %s", int(s.bits+.5), s.strength)
	printAt(screen, r.x+12, meterY, strength, s.colors.muted)

	lengthValue := s.lengthText
	lengthLabelColor := s.colors.muted
	if s.selected == focusLength {
		lengthValue = "[" + lengthValue + "]"
		lengthLabelColor = s.colors.focus
	}
	lengthLabel := "Length: " + lengthValue
	printAt(screen, r.x+2, lengthLabelY, lengthLabel, lengthLabelColor)
	s.drawLength(screen, l.length, ox, oy)
	if !l.compact {
		printAt(screen, r.x+2, charactersY, "Characters", s.colors.muted)
	}
	s.drawCheck(screen, l.upper, ox, oy, focusUpper, "Uppercase", s.cfg.Upper)
	s.drawCheck(screen, l.lower, ox, oy, focusLower, "Lowercase", s.cfg.Lower)
	s.drawCheck(screen, l.numbers, ox, oy, focusNumbers, "Numbers", s.cfg.Numbers)
	s.drawCheck(screen, l.symbols, ox, oy, focusSymbols, "Symbols", s.cfg.Symbols)
	ambigLabel := "Exclude ambiguous (I l 1 O 0)"
	if l.compact {
		ambigLabel = "Exclude ambiguous"
	}
	s.drawCheck(screen, l.ambig, ox, oy, focusAmbiguous, ambigLabel, s.cfg.ExcludeAmbiguous)
}

func (s *screen) drawAction(screen tcell.Screen, r rect, ox, oy int, f focus, label string) {
	r.x += ox
	r.y += oy
	color := s.colors.muted
	if s.selected == f {
		color = s.colors.focus
	}
	printAt(screen, r.x, r.y, label, color)
}

func (s *screen) drawLength(screen tcell.Screen, r rect, ox, oy int) {
	r.x += ox
	r.y += oy
	color := s.colors.text
	if s.selected == focusLength {
		color = s.colors.focus
	}
	printAt(screen, r.x, r.y, "4", s.colors.muted)
	railStart, railWidth := sliderRail(r)
	lengthValue := s.cfg.Length
	if parsed, err := strconv.Atoi(s.lengthText); err == nil {
		lengthValue = parsed
	}
	knob := railStart + (lengthValue-generator.MinLength)*(railWidth-1)/(generator.MaxLength-generator.MinLength)
	for i := 0; i < railWidth; i++ {
		ch := '─'
		if railStart+i <= knob {
			ch = '━'
		}
		screen.SetContent(railStart+i, r.y, ch, nil, tcell.StyleDefault.Foreground(color).Background(s.colors.surface))
	}
	screen.SetContent(knob, r.y, '●', nil, tcell.StyleDefault.Foreground(color).Background(s.colors.surface))
	printAt(screen, railStart+railWidth+1, r.y, "256", s.colors.muted)
}

func (s *screen) drawCheck(screen tcell.Screen, r rect, ox, oy int, f focus, label string, checked bool) {
	r.x += ox
	r.y += oy
	mark, color := "[ ]", s.colors.text
	if checked {
		mark = "[✓]"
	}
	if s.selected == f {
		color = s.colors.focus
	}
	printAt(screen, r.x, r.y, mark+" "+label, color)
	initialX := r.x + 4
	cell, style, _ := screen.Get(initialX, r.y)
	initial, size := utf8.DecodeRuneInString(cell)
	var combining []rune
	if size < len(cell) {
		combining = []rune(cell[size:])
	}
	screen.SetContent(initialX, r.y, initial, combining, style.Underline(true))
}

func (s *screen) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return s.WrapInputHandler(func(event *tcell.EventKey, _ func(tview.Primitive)) {
		if s.themePanel.open {
			s.handleThemeInput(event)
			return
		}
		if s.storeForm {
			s.handleStoreInput(event)
			return
		}
		if s.route == VaultRoute && s.vaultFiltering {
			s.handleVaultFilterInput(event)
			return
		}
		if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && event.Rune() == 'q') {
			s.save()
			s.quit()
			return
		}
		if event.Key() == tcell.KeyRune && event.Rune() == 'v' {
			s.switchRoute()
			return
		}
		if s.route == VaultRoute {
			s.handleVaultInput(event)
			return
		}
		switch event.Key() {
		case tcell.KeyTAB, tcell.KeyDown:
			s.move(1)
			return
		case tcell.KeyBacktab, tcell.KeyUp:
			s.move(-1)
			return
		case tcell.KeyLeft:
			s.selected = focusLength
			s.adjustLength(-1)
			return
		case tcell.KeyRight:
			s.selected = focusLength
			s.adjustLength(1)
			return
		case tcell.KeyEnter:
			s.activate()
			return
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if s.selected == focusLength && len(s.lengthText) > 0 {
				s.lengthText = s.lengthText[:len(s.lengthText)-1]
			}
			return
		}
		if event.Key() != tcell.KeyRune {
			return
		}
		switch event.Rune() {
		case 'U', 'u':
			s.selected = focusUpper
			s.activate()
		case 'L', 'l':
			s.selected = focusLower
			s.activate()
		case 'N', 'n':
			s.selected = focusNumbers
			s.activate()
		case 'S', 's':
			s.selected = focusSymbols
			s.activate()
		case 'a':
			s.openStoreForm()
		case 'E', 'e':
			s.selected = focusAmbiguous
			s.activate()
		case 'j':
			s.move(1)
		case 'k':
			s.move(-1)
		case '-':
			s.selected = focusLength
			s.adjustLength(-1)
		case '+':
			s.selected = focusLength
			s.adjustLength(1)
		case ' ':
			s.activate()
		case 'r':
			_ = s.regenerate()
		case 'c':
			s.copy()
		case 't':
			s.openThemePanel()
		default:
			if s.selected == focusLength && event.Rune() >= '0' && event.Rune() <= '9' {
				s.typeLength(event.Rune())
			}
		}
	})
}

func (s *screen) switchRoute() {
	if s.route == GeneratorRoute {
		s.route = VaultRoute
		s.refreshVault()
		return
	}
	s.route = GeneratorRoute
}

func (s *screen) refreshVault() {
	s.vaultError = ""
	nodes, err := s.vault.List(context.Background(), s.vaultPath)
	if err != nil {
		if s.vault.Provider == nil {
			return
		}
		debug.Failure("vault list", vaultFailureCause(err))
		s.vaultError = "Vault unavailable or folder cannot be opened."
		nodes = nil
	}
	s.vaultNodes = nodes
	s.vaultSelected = min(s.vaultSelected, max(0, len(s.vaultDisplayNodes())-1))
}

func (s *screen) handleVaultInput(event *tcell.EventKey) {
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
			s.openThemePanel()
		}
	}
}

func (s *screen) handleVaultFilterInput(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		s.vaultFiltering, s.vaultFilter, s.vaultSelected = false, "", 0
	case tcell.KeyCtrlU:
		s.vaultFilter, s.vaultSelected = "", 0
	case tcell.KeyCtrlW:
		s.vaultFilter, s.vaultSelected = deletePreviousWord(s.vaultFilter), 0
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(s.vaultFilter) > 0 {
			runes := []rune(s.vaultFilter)
			s.vaultFilter, s.vaultSelected = string(runes[:len(runes)-1]), 0
		}
	case tcell.KeyUp:
		s.moveVaultSelection(-1)
	case tcell.KeyDown:
		s.moveVaultSelection(1)
	case tcell.KeyEnter:
		s.openVaultNode()
	case tcell.KeyRune:
		if event.Rune() >= ' ' {
			s.vaultFilter += string(event.Rune())
			s.vaultSelected = 0
		}
	}
}

func (s *screen) moveVaultSelection(delta int) {
	nodes := s.vaultDisplayNodes()
	s.vaultSelected = min(max(0, len(nodes)-1), max(0, s.vaultSelected+delta))
}

func (s *screen) vaultParent() {
	if len(s.vaultPath) == 0 {
		return
	}
	child := s.vaultPath
	s.vaultFiltering, s.vaultFilter = false, ""
	s.vaultPath = s.vaultPath[:len(s.vaultPath)-1]
	s.refreshVault()
	for i, node := range s.vaultDisplayNodes() {
		if !s.isVaultParentNode(node) && node.Path.String() == child.String() {
			s.vaultSelected = i
			return
		}
	}
}

func (s *screen) openVaultNode() {
	node, ok := s.selectedVaultNode()
	if !ok {
		return
	}
	if s.isVaultParentNode(node) {
		s.vaultParent()
		return
	}
	if node.Kind == vault.FolderNode {
		s.vaultPath, s.vaultSelected, s.vaultFiltering, s.vaultFilter = node.Path, 1, false, ""
		s.refreshVault()
		return
	}
	s.copyVaultEntry()
}

// runPrompted executes fn with the terminal released while the app is
// running, so subprocesses that need pinentry get a usable TTY. It falls
// back to a direct call when suspension is unavailable.
func (s *screen) runPrompted(fn func() error) error {
	if s.suspend == nil {
		return fn()
	}
	var err error
	if !s.suspend(func() { err = fn() }) {
		return fn()
	}
	return err
}

func (s *screen) copyVaultEntry() {
	node, ok := s.selectedVaultNode()
	if !ok || node.Kind != vault.EntryNode {
		return
	}
	if err := s.runPrompted(func() error { return s.vault.CopyPassword(context.Background(), node.Path) }); err != nil {
		var diagnostic vault.Diagnostic
		if !errors.As(err, &diagnostic) {
			debug.Failure("vault copy", vaultFailureCause(err))
		}
		s.notify(notificationError, copyErrorMessage(err))
		return
	}
	s.notify(notificationSuccess, "Copied to clipboard")
}

func (s *screen) selectedVaultNode() (vault.Node, bool) {
	nodes := s.vaultDisplayNodes()
	if s.vaultSelected < 0 || s.vaultSelected >= len(nodes) {
		return vault.Node{}, false
	}
	return nodes[s.vaultSelected], true
}

func (s *screen) openStoreForm() {
	if s.vault.Provider == nil {
		s.notify(notificationWarning, "Vault unavailable")
		return
	}
	s.storeForm, s.storePath, s.storeSelected, s.storeMessage = true, "", 0, ""
	s.refreshStoreSuggestions()
}

func (s *screen) handleStoreInput(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		s.storeForm, s.storePath, s.storeNodes, s.storeSelected, s.storeMessage = false, "", nil, 0, ""
	case tcell.KeyCtrlU:
		s.storePath = ""
		s.refreshStoreSuggestions()
	case tcell.KeyCtrlW:
		s.storePath = deletePreviousWord(s.storePath)
		s.refreshStoreSuggestions()
	case tcell.KeyUp:
		s.storeSelected = max(0, s.storeSelected-1)
	case tcell.KeyDown:
		s.storeSelected = min(max(0, len(s.storeNodes)-1), s.storeSelected+1)
	case tcell.KeyTAB:
		s.insertStoreSuggestion()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(s.storePath) > 0 {
			runes := []rune(s.storePath)
			s.storePath = string(runes[:len(runes)-1])
			s.refreshStoreSuggestions()
		}
	case tcell.KeyEnter:
		path, err := vaultPath(s.storePath)
		if err != nil || len(path) == 0 {
			s.notify(notificationWarning, "Enter a valid vault destination")
			return
		}
		if err := s.vault.Store(context.Background(), vault.Entry{Path: path, Password: s.password}); err != nil {
			var diagnostic vault.Diagnostic
			if !errors.As(err, &diagnostic) {
				debug.Failure("vault store", vaultFailureCause(err))
			}
			s.notify(notificationError, storeErrorMessage(err))
			return
		}
		s.storeForm, s.storePath, s.storeNodes, s.storeSelected, s.storeMessage = false, "", nil, 0, ""
		s.notify(notificationSuccess, "Stored in vault")
	case tcell.KeyRune:
		if event.Rune() >= ' ' {
			s.storePath += string(event.Rune())
			s.refreshStoreSuggestions()
		}
	}
}

func (s *screen) refreshStoreSuggestions() {
	s.storeNodes, s.storeSelected, s.storeMessage = nil, 0, ""
	path, prefix, err := storeSuggestionQuery(s.storePath)
	if err != nil {
		return
	}
	nodes, err := s.vault.List(context.Background(), path)
	if err != nil {
		if !errors.Is(err, vault.ErrNotFound) {
			debug.Failure("vault suggestions", vaultFailureCause(err))
			s.storeMessage = "Folders cannot be loaded. Type a destination instead."
		}
		return
	}
	for _, node := range nodes {
		if node.Kind == vault.FolderNode && strings.HasPrefix(strings.ToLower(node.Name), strings.ToLower(prefix)) {
			s.storeNodes = append(s.storeNodes, node)
		}
	}
}

func storeSuggestionQuery(destination string) (vault.Path, string, error) {
	if destination == "" {
		return nil, "", nil
	}
	separator := strings.LastIndex(destination, "/")
	if separator < 0 {
		return nil, destination, nil
	}
	parent, prefix := destination[:separator], destination[separator+1:]
	if parent == "" {
		return nil, prefix, nil
	}
	path, err := vaultPath(parent)
	return path, prefix, err
}

func deletePreviousWord(value string) string {
	runes := []rune(value)
	for len(runes) > 0 && unicode.IsSpace(runes[len(runes)-1]) {
		runes = runes[:len(runes)-1]
	}
	for len(runes) > 0 && !unicode.IsSpace(runes[len(runes)-1]) && runes[len(runes)-1] != '/' {
		runes = runes[:len(runes)-1]
	}
	for len(runes) > 0 && unicode.IsSpace(runes[len(runes)-1]) {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

func (s *screen) insertStoreSuggestion() {
	if len(s.storeNodes) == 0 {
		return
	}
	s.storePath = s.storeNodes[s.storeSelected].Path.String() + "/"
	s.refreshStoreSuggestions()
}

func copyErrorMessage(err error) string {
	var diagnostic vault.Diagnostic
	if errors.As(err, &diagnostic) {
		return diagnostic.UserMessage()
	}
	switch {
	case errors.Is(err, vault.ErrLocked):
		return "Vault locked (no details)"
	case errors.Is(err, vault.ErrNotFound):
		return "Vault entry not found"
	case errors.Is(err, vault.ErrMalformed):
		return "Vault entry format invalid"
	case errors.Is(err, vault.ErrUninitialized):
		return "Vault not initialized"
	case errors.Is(err, vault.ErrUnavailable):
		return "Vault unavailable"
	case errors.Is(err, service.ErrClipboard):
		return "Clipboard unavailable"
	default:
		return "Copy failed"
	}
}

func vaultFailureCause(err error) debug.Cause {
	switch {
	case errors.Is(err, vault.ErrLocked):
		return debug.Locked
	case errors.Is(err, vault.ErrNotFound):
		return debug.NotFound
	case errors.Is(err, vault.ErrUninitialized), errors.Is(err, vault.ErrInvalidPath), errors.Is(err, vault.ErrMalformed):
		return debug.Invalid
	case errors.Is(err, vault.ErrUnavailable):
		return debug.Unavailable
	case errors.Is(err, vault.ErrConflict):
		return debug.Conflict
	case errors.Is(err, service.ErrClipboard):
		return debug.Clipboard
	default:
		return debug.Unknown
	}
}

func storeErrorMessage(err error) string {
	var diagnostic vault.Diagnostic
	if errors.As(err, &diagnostic) {
		return diagnostic.UserMessage()
	}
	switch {
	case errors.Is(err, vault.ErrUninitialized):
		return "Vault is not initialized. Run: pass init <recipient>"
	case errors.Is(err, vault.ErrLocked):
		return "Vault locked (no details)"
	case errors.Is(err, vault.ErrUnavailable):
		return "Vault is unavailable. Check pass configuration."
	default:
		return "Store failed"
	}
}

func vaultPath(text string) (vault.Path, error) {
	path := vault.Path(strings.Split(text, "/"))
	if err := path.Validate(); err != nil {
		return nil, err
	}
	return path, nil
}

func (s *screen) move(delta int) {
	_, _, width, height := s.GetRect()
	l := calculateLayout(width, height)
	for {
		s.selected = focus((int(s.selected) + delta + int(focusCount)) % int(focusCount))
		if !l.compact || s.selected != focusRegen {
			return
		}
	}
}
func (s *screen) adjustLength(delta int) { s.setLength(s.cfg.Length + delta) }
func (s *screen) typeLength(r rune) {
	if len(s.lengthText) >= 3 {
		s.lengthText = ""
	}
	s.lengthText += string(r)
	if n, err := strconv.Atoi(s.lengthText); err == nil && n >= generator.MinLength && n <= generator.MaxLength {
		s.setLength(n)
	}
}
func (s *screen) setLength(n int) {
	n = max(generator.MinLength, min(generator.MaxLength, n))
	s.cfg.Length, s.lengthText = n, strconv.Itoa(n)
	_ = s.regenerate()
	s.save()
}

func (s *screen) activate() {
	switch s.selected {
	case focusUpper:
		s.cfg.Upper = !s.cfg.Upper
	case focusLower:
		s.cfg.Lower = !s.cfg.Lower
	case focusNumbers:
		s.cfg.Numbers = !s.cfg.Numbers
	case focusSymbols:
		s.cfg.Symbols = !s.cfg.Symbols
	case focusAmbiguous:
		s.cfg.ExcludeAmbiguous = !s.cfg.ExcludeAmbiguous
	case focusStore:
		s.openStoreForm()
		return
	case focusRegen:
		_ = s.regenerate()
		return
	case focusCopy:
		s.copy()
		return
	case focusLength:
		return
	}
	if !s.cfg.Upper && !s.cfg.Lower && !s.cfg.Numbers && !s.cfg.Symbols {
		switch s.selected {
		case focusUpper:
			s.cfg.Upper = true
		case focusLower:
			s.cfg.Lower = true
		case focusNumbers:
			s.cfg.Numbers = true
		case focusSymbols:
			s.cfg.Symbols = true
		}
		s.notify(notificationWarning, "Enable at least one character type")
		return
	}
	_ = s.regenerate()
	s.save()
}
func (s *screen) copy() {
	if s.onCopy != nil {
		if err := s.onCopy(s.password); err != nil {
			debug.Failure("generator copy", debug.Clipboard)
			s.notify(notificationError, "Clipboard unavailable")
			return
		}
	}
	s.notify(notificationSuccess, "Copied to clipboard")
}

func (s *screen) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return s.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, _ func(tview.Primitive)) (bool, tview.Primitive) {
		if s.themePanel.open {
			return true, s
		}
		if s.storeForm {
			return s.handleStoreMouse(action, event)
		}
		x, y := event.Position()
		ox, oy, w, h := s.GetRect()
		l := calculateLayout(w, h)
		if l.tooSmall {
			return true, s
		}
		if s.route == VaultRoute {
			return s.handleVaultMouse(action, event, l, ox, oy)
		}
		if action == tview.MouseMove && s.draggingSlider {
			s.setLengthFromPointer(l.length, ox, oy, x)
			return true, s
		}
		if action == tview.MouseLeftUp {
			s.draggingSlider = false
			return true, nil
		}
		if action != tview.MouseLeftDown && action != tview.MouseLeftClick {
			return false, nil
		}
		r := l.length
		r.x += ox
		r.y += oy
		if action == tview.MouseLeftDown {
			if !r.contains(x, y) {
				return false, nil
			}
			s.selected = focusLength
			s.draggingSlider = true
			s.setLengthFromPointer(l.length, ox, oy, x)
			return true, s
		}
		click := func(r rect, f focus) bool {
			r.x += ox
			r.y += oy
			if r.contains(x, y) {
				s.selected = f
				s.activate()
				return true
			}
			return false
		}
		if click(l.upper, focusUpper) || click(l.lower, focusLower) || click(l.numbers, focusNumbers) || click(l.symbols, focusSymbols) || click(l.ambig, focusAmbiguous) || click(l.store, focusStore) || (!l.compact && click(l.regen, focusRegen)) || click(l.copy, focusCopy) {
			return true, s
		}
		if r.contains(x, y) {
			s.selected = focusLength
			s.setLengthFromPointer(l.length, ox, oy, x)
			return true, s
		}
		return false, nil
	})
}

func (s *screen) handleVaultMouse(action tview.MouseAction, event *tcell.EventMouse, l layout, ox, oy int) (bool, tview.Primitive) {
	s.draggingSlider = false
	switch action {
	case tview.MouseScrollUp:
		s.moveVaultSelection(-1)
		return true, s
	case tview.MouseScrollDown:
		s.moveVaultSelection(1)
		return true, s
	case tview.MouseRightClick, tview.MouseRightDoubleClick:
		s.vaultParent()
		return true, s
	case tview.MouseLeftDown, tview.MouseLeftUp, tview.MouseMove:
		return true, s
	case tview.MouseLeftDoubleClick:
		x, y := event.Position()
		index := s.vaultRowAt(x, y, l, ox, oy)
		if index < 0 {
			return false, nil
		}
		s.vaultSelected = index
		s.openVaultNode()
		return true, s
	case tview.MouseLeftClick:
		x, y := event.Position()
		index := s.vaultRowAt(x, y, l, ox, oy)
		if index < 0 {
			return false, nil
		}
		if index == s.vaultSelected {
			s.openVaultNode()
			return true, s
		}
		s.vaultSelected = index
		return true, s
	default:
		return false, nil
	}
}

func (s *screen) vaultRowAt(x, y int, l layout, ox, oy int) int {
	if s.vault.Provider == nil {
		return -1
	}
	nodes := s.vaultDisplayNodes()
	if len(nodes) == 0 {
		return -1
	}
	r := l.card
	r.x += ox
	r.y += oy
	if x < r.x+1 || x >= r.x+r.w-1 {
		return -1
	}
	listY, limit, start := s.vaultListWindow(r)
	if limit <= 0 {
		return -1
	}
	offset := y - listY
	if offset < 0 || offset >= limit {
		return -1
	}
	index := start + offset
	if index < 0 || index >= len(nodes) {
		return -1
	}
	return index
}

func (s *screen) handleStoreMouse(action tview.MouseAction, event *tcell.EventMouse) (bool, tview.Primitive) {
	if action != tview.MouseLeftDown && action != tview.MouseLeftClick {
		return true, s
	}
	x, y := event.Position()
	ox, oy, width, height := s.GetRect()
	r, visible := s.storeFormRect(ox, oy, width, height)
	row := y - (r.y + 6)
	if x < r.x+2 || x >= r.x+r.w-2 || row < 0 || row >= visible {
		return true, s
	}
	if action == tview.MouseLeftClick && row == s.storeSelected {
		s.insertStoreSuggestion()
		return true, s
	}
	s.storeSelected = row
	return true, s
}

func sliderRail(r rect) (start, width int) { return r.x + 3, max(8, r.w-8) }

func (s *screen) setLengthFromPointer(r rect, ox, oy, pointerX int) {
	r.x += ox
	r.y += oy
	railStart, railWidth := sliderRail(r)
	pointerX = max(railStart, min(railStart+railWidth-1, pointerX))
	n := generator.MinLength + (pointerX-railStart)*(generator.MaxLength-generator.MinLength)/(railWidth-1)
	s.setLength(n)
}

func printAt(screen tcell.Screen, x, y int, text string, color tcell.Color) {
	tview.Print(screen, tview.Escape(text), x, y, len(text), tview.AlignLeft, color)
}
func truncate(value string, width int) string {
	if uniseg.StringWidth(value) <= width {
		return value
	}
	if width < 2 {
		return ""
	}
	var result strings.Builder
	for _, r := range value {
		if uniseg.StringWidth(result.String()+string(r)) > width-1 {
			break
		}
		result.WriteRune(r)
	}
	return result.String() + "…"
}
func drawBox(screen tcell.Screen, r rect, color, surface tcell.Color) {
	style := tcell.StyleDefault.Foreground(color).Background(surface)
	for yy := r.y + 1; yy < r.y+r.h-1; yy++ {
		for xx := r.x + 1; xx < r.x+r.w-1; xx++ {
			screen.SetContent(xx, yy, ' ', nil, style)
		}
	}
	for xx := r.x + 1; xx < r.x+r.w-1; xx++ {
		screen.SetContent(xx, r.y, '─', nil, style)
		screen.SetContent(xx, r.y+r.h-1, '─', nil, style)
	}
	for yy := r.y + 1; yy < r.y+r.h-1; yy++ {
		screen.SetContent(r.x, yy, '│', nil, style)
		screen.SetContent(r.x+r.w-1, yy, '│', nil, style)
	}
	screen.SetContent(r.x, r.y, '╭', nil, style)
	screen.SetContent(r.x+r.w-1, r.y, '╮', nil, style)
	screen.SetContent(r.x, r.y+r.h-1, '╰', nil, style)
	screen.SetContent(r.x+r.w-1, r.y+r.h-1, '╯', nil, style)
}
