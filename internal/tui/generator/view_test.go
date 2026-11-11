package generator

import (
	"errors"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"lazypass/internal/config"
	"lazypass/internal/tui/shared"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestActionClickDoesNotRetainMouseCapture(t *testing.T) {
	opened := false
	v := New(config.Defaults().ToOptions(), Dependencies{OpenStore: func() { opened = true }})
	frame := testFrame(false, false)
	l := calculateLayout(frame)
	result := v.HandleMouse(tview.MouseLeftClick, tcell.NewEventMouse(l.store.x, l.store.y, tcell.Button1, tcell.ModNone), frame)
	if !opened || !result.Consumed || result.Capture {
		t.Fatal("Add action retained mouse capture after opening overlay")
	}
}

func testFrame(compact, short bool) shared.Frame {
	w, h := 72, 17
	if compact {
		w, h = 48, 16
	} else if short {
		h = 13
	}
	return shared.Frame{Viewport: shared.Rect{W: w + 2, H: h + 2}, Card: shared.Rect{X: 1, Y: 1, W: w, H: h}, Compact: compact, Short: short, Palette: shared.DefaultPalette()}
}

func TestRegenerateProducesPassword(t *testing.T) {
	v := New(config.Defaults().ToOptions(), Dependencies{})
	if err := v.Regenerate(); err != nil {
		t.Fatal(err)
	}
	if len(v.Password()) != 20 {
		t.Fatal("wrong password length")
	}
}

func TestToggleKeepsOneClass(t *testing.T) {
	options := config.Defaults().ToOptions()
	options.Upper, options.Lower, options.Numbers, options.Symbols = true, false, false, false
	v := New(options, Dependencies{})
	_ = v.Regenerate()
	before := v.Password()
	v.selected = focusUpper
	v.activate()
	if !v.Options().Upper || v.Password() != before {
		t.Fatal("last character class toggle was not rejected")
	}
}

func TestLengthTypingSupportsMoreThanFifty(t *testing.T) {
	v := New(config.Defaults().ToOptions(), Dependencies{})
	v.lengthText = ""
	for _, r := range "128" {
		v.typeLength(r)
	}
	if v.Options().Length != 128 {
		t.Fatal("length typing failed")
	}
}

func TestGlobalOptionShortcuts(t *testing.T) {
	v := New(config.Defaults().ToOptions(), Dependencies{})
	v.selected = focusSymbols
	v.HandleKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone), testFrame(false, false))
	if v.selected != focusLength || v.Options().Length != 21 {
		t.Fatal("Right did not adjust and focus length")
	}
	for _, r := range "UL" {
		v.HandleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModShift), testFrame(false, false))
	}
	if v.Options().Upper || v.Options().Lower {
		t.Fatal("uppercase shortcuts failed")
	}
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone), testFrame(false, false))
	if !v.Options().Lower {
		t.Fatal("lowercase shortcut failed")
	}
}

func TestSliderPointerClampsAndSetsLength(t *testing.T) {
	v := New(config.Defaults().ToOptions(), Dependencies{})
	f := testFrame(false, false)
	f.Card = f.Card.Translate(8, 3)
	l := calculateLayout(f)
	start, width := sliderRail(l.length)
	v.HandleMouse(tview.MouseLeftDown, tcell.NewEventMouse(start+width-1, l.length.y, tcell.Button1, tcell.ModNone), f)
	if v.Options().Length != 256 || !v.draggingSlider {
		t.Fatal("slider endpoint/capture failed")
	}
	v.HandleMouse(tview.MouseMove, tcell.NewEventMouse(start-20, l.length.y, tcell.Button1, tcell.ModNone), f)
	if v.Options().Length != 4 {
		t.Fatal("slider did not clamp")
	}
	v.ReleaseMouse()
	if v.draggingSlider {
		t.Fatal("drag state not released")
	}
}

func TestGeneratorRenderControls(t *testing.T) {
	for _, compact := range []bool{false, true} {
		for _, exclude := range []bool{false, true} {
			options := config.Defaults().ToOptions()
			options.ExcludeAmbiguous = exclude
			v := New(options, Dependencies{})
			_ = v.Regenerate()
			f := testFrame(compact, false)
			sim := tcell.NewSimulationScreen("UTF-8")
			if err := sim.Init(); err != nil {
				t.Fatal(err)
			}
			sim.SetSize(84, 24)
			v.Draw(sim, f)
			l := calculateLayout(f)
			for _, test := range []struct {
				r       rect
				initial rune
			}{{l.upper, 'U'}, {l.lower, 'L'}, {l.numbers, 'N'}, {l.symbols, 'S'}, {l.ambig, 'E'}} {
				cell, style, _ := sim.Get(test.r.x+4, test.r.y)
				initial, _ := utf8.DecodeRuneInString(cell)
				fg, _, attrs := style.Decompose()
				if initial != test.initial || attrs&tcell.AttrUnderline == 0 || fg != f.Palette.Text {
					t.Fatal("shortcut underline/color misaligned")
				}
			}
			var row strings.Builder
			for x := l.ambig.x; x < l.ambig.x+l.ambig.w; x++ {
				cell, _, _ := sim.Get(x, l.ambig.y)
				row.WriteString(cell)
			}
			mark := "[ ]"
			if exclude {
				mark = "[✓]"
			}
			if !strings.Contains(row.String(), mark+" Exclude ambiguous") || (compact && strings.Contains(row.String(), "(I")) {
				t.Fatalf("ambiguous label: %q", row.String())
			}
			v.selected = focusSymbols
			v.Draw(sim, f)
			_, style, _ := sim.Get(l.symbols.x+4, l.symbols.y)
			fg, _, attrs := style.Decompose()
			if fg != f.Palette.Focus || attrs&tcell.AttrUnderline == 0 {
				t.Fatal("focus underline/color missing")
			}
			v.HandleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone), f)
			v.Draw(sim, f)
			cell, style, _ := sim.Get(l.card.x+2, l.card.y+6)
			fg, _, _ = style.Decompose()
			if cell != "L" || fg != f.Palette.Focus {
				t.Fatal("length focus not rendered")
			}
			sim.Fini()
		}
	}
}

func TestGeneratorFooterShowsOnlyFocusedControlHint(t *testing.T) {
	v := New(config.Defaults().ToOptions(), Dependencies{})
	for _, width := range []int{120, 84, 50, 32} {
		for _, test := range []struct {
			focus      focus
			want, omit string
		}{{focusLength, "←/→ length", "<space> toggle"}, {focusSymbols, "<space> toggle", "←/→ length"}, {focusStore, "", "<space> toggle"}} {
			v.selected = test.focus
			footer := v.Footer(width)
			if utf8.RuneCountInString(footer) > width-2 || !strings.Contains(footer, test.want) || strings.Contains(footer, test.omit) {
				t.Fatalf("footer: %q", footer)
			}
			if strings.Contains(footer, "t themes") && (strings.Index(footer, "c copy") >= strings.Index(footer, "t themes") || strings.Index(footer, "t themes") >= strings.Index(footer, "v view")) {
				t.Fatal("footer ordering changed")
			}
		}
	}
}

func TestCopyStatusReflectsClipboardOutcome(t *testing.T) {
	for _, failure := range []bool{false, true} {
		var level shared.Level
		var message string
		v := New(config.Defaults().ToOptions(), Dependencies{Copy: func(string) error {
			if failure {
				return errors.New("secret external error")
			}
			return nil
		}, Notify: func(l shared.Level, m string) { level, message = l, m }})
		v.copy()
		if failure {
			if level != shared.Error || message != "Clipboard unavailable" {
				t.Fatal("unsafe failure status")
			}
		} else if level != shared.Success || message != "Copied to clipboard" {
			t.Fatal("success status missing")
		}
	}
}
