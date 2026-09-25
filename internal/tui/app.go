package tui

import (
	"fmt"
	"strconv"
	"time"
	"unicode/utf8"

	"lazypass/internal/app"
	"lazypass/internal/config"
	"lazypass/internal/generator"
	"lazypass/internal/theme"

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
	focusRegen
	focusCopy
	focusCount
)

const (
	fullFooter    = "←/→ length  •  U/L/N/S/E toggle  •  Space toggle  •  t themes  •  r regenerate  •  c copy  •  q quit"
	compactFooter = "Tab focus  •  Space toggle  •  t themes  •  r regenerate  •  c copy  •  q quit"
	shortFooter   = "t themes  •  r regenerate  •  c copy  •  q quit"
	minimalFooter = "c copy  •  q quit"
)

// App owns application lifecycle; Screen is responsible for all viewport drawing.
type App struct {
	app    *tview.Application
	screen *screen
}

func NewApp(cfg config.Config, cfgPath string, onCopy func(string) error) *App {
	colors := defaultTUIPalette()
	s := &screen{Box: tview.NewBox(), cfg: cfg, cfgPath: cfgPath, onCopy: onCopy, selected: focusLength, colors: colors}
	if palette, err := theme.Load(cfg.Theme); err == nil {
		if loaded, err := newTUIPalette(palette); err == nil {
			s.colors = loaded
		}
	} else {
		s.setStatus(fmt.Sprintf("Theme %q unavailable; using %s", cfg.Theme, theme.DefaultName()))
	}
	s.lengthText = strconv.Itoa(cfg.Length)
	app := tview.NewApplication().EnableMouse(true)
	s.quit = app.Stop
	return &App{app: app, screen: s}
}

func (a *App) CurrentPassword() string           { return a.screen.password }
func (a *App) CurrentOptions() generator.Options { return a.screen.cfg.ToOptions() }

func (a *App) Init() error {
	if err := a.screen.regenerate(); err != nil {
		return err
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
	quit           func()
	draggingSlider bool
	colors         tuiPalette
	themePanel     themePanel
}

func (s *screen) regenerate() error {
	results, err := app.Generate(s.cfg.ToOptions(), 1)
	if err != nil {
		return err
	}
	s.password, s.bits, s.strength = results[0].Password, results[0].EntropyBits, results[0].Strength
	return nil
}

func (s *screen) save() { _ = s.cfg.Save(s.cfgPath) }

func (s *screen) setStatus(message string) {
	s.status, s.statusTill = message, time.Now().Add(2*time.Second)
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
		printAt(screen, x+max(1, (width-29)/2), y+height/2, "Resize terminal to at least 32 x 10", s.colors.warning)
		return
	}
	logo := calculateLogoLayout(width, height)
	drawLogo(screen, logo, x, y, s.colors)
	s.drawHeaderStatus(screen, logo, x, y, width)
	s.drawCard(screen, l, x, y)
	footer := fullFooter
	if l.compact || l.short {
		footer = compactFooter
	}
	for _, fallback := range []string{compactFooter, shortFooter, minimalFooter} {
		if utf8.RuneCountInString(footer) > width-2 {
			footer = fallback
		}
	}
	printAt(screen, x+max(1, (width-utf8.RuneCountInString(footer))/2), y+height-1, footer, s.colors.muted)
	if s.themePanel.open {
		s.drawThemePanel(screen, x, y, width, height)
	}
}

func (s *screen) drawHeaderStatus(screen tcell.Screen, logo logoLayout, x, y, width int) {
	if s.status != "" && time.Now().Before(s.statusTill) {
		statusX := x + width - utf8.RuneCountInString(s.status) - 2
		if statusX > x+logo.right()+2 {
			printAt(screen, statusX, y, s.status, s.colors.focus)
		}
	}
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
	pw := truncate(s.password, max(12, r.w-34))
	printAt(screen, r.x+2, passwordY, pw, s.colors.text)
	s.drawAction(screen, l.regen, ox, oy, focusRegen, "↻ Regenerate")
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
	s.drawCheck(screen, l.upper, ox, oy, focusUpper, "Uppercase", "U", s.cfg.Upper)
	s.drawCheck(screen, l.lower, ox, oy, focusLower, "Lowercase", "L", s.cfg.Lower)
	s.drawCheck(screen, l.numbers, ox, oy, focusNumbers, "Numbers", "N", s.cfg.Numbers)
	s.drawCheck(screen, l.symbols, ox, oy, focusSymbols, "Symbols", "S", s.cfg.Symbols)
	ambigLabel := "Exclude ambiguous (I l 1 O 0)"
	if l.compact {
		ambigLabel = "Exclude ambiguous"
	}
	s.drawCheck(screen, l.ambig, ox, oy, focusAmbiguous, ambigLabel, "E", s.cfg.ExcludeAmbiguous)
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

func (s *screen) drawCheck(screen tcell.Screen, r rect, ox, oy int, f focus, label, shortcut string, checked bool) {
	r.x += ox
	r.y += oy
	mark, color := "[ ]", s.colors.muted
	if checked {
		mark, color = "[✓]", s.colors.accent
	}
	if s.selected == f {
		color = s.colors.focus
	}
	printAt(screen, r.x, r.y, mark+" "+label, color)
	badgeColor := s.colors.muted
	if s.selected == f {
		badgeColor = s.colors.focus
	}
	// Rune widths, not bytes: "✓" is 3 bytes but 1 cell, and using len()
	// would shift the badge every time the box is toggled.
	textWidth := utf8.RuneCountInString(mark) + 1 + utf8.RuneCountInString(label)
	badgeX := r.x + 18
	if badgeX < r.x+textWidth+2 {
		badgeX = r.x + textWidth + 2
	}
	printAt(screen, badgeX, r.y, "["+shortcut+"]", badgeColor)
}

func (s *screen) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return s.WrapInputHandler(func(event *tcell.EventKey, _ func(tview.Primitive)) {
		if s.themePanel.open {
			s.handleThemeInput(event)
			return
		}
		if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && event.Rune() == 'q') {
			s.save()
			s.quit()
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

func (s *screen) move(delta int) {
	s.selected = focus((int(s.selected) + delta + int(focusCount)) % int(focusCount))
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
		s.setStatus("Enable at least one character type")
		return
	}
	_ = s.regenerate()
	s.save()
}
func (s *screen) copy() {
	if s.onCopy != nil {
		if err := s.onCopy(s.password); err != nil {
			s.setStatus("Copy failed")
			return
		}
	}
	s.setStatus("Copied to clipboard")
}

func (s *screen) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return s.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, _ func(tview.Primitive)) (bool, tview.Primitive) {
		if s.themePanel.open {
			return true, s
		}
		x, y := event.Position()
		ox, oy, w, h := s.GetRect()
		l := calculateLayout(w, h)
		if l.tooSmall {
			return true, s
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
		if click(l.upper, focusUpper) || click(l.lower, focusLower) || click(l.numbers, focusNumbers) || click(l.symbols, focusSymbols) || click(l.ambig, focusAmbiguous) || click(l.regen, focusRegen) || click(l.copy, focusCopy) {
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
	if len(value) <= width {
		return value
	}
	if width < 2 {
		return value[:width]
	}
	return value[:width-1] + "…"
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
