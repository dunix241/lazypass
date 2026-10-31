package generator

import (
	"lazypass/internal/generator"
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (s *View) HandleMouse(action tview.MouseAction, event *tcell.EventMouse, frame shared.Frame) shared.MouseResult {
	l := calculateLayout(frame)
	x, y := event.Position()
	if action == tview.MouseMove && s.draggingSlider {
		s.setLengthFromPointer(l.length, 0, 0, x)
		return shared.MouseResult{Consumed: true, Capture: true}
	}
	if action == tview.MouseLeftUp {
		s.ReleaseMouse()
		return shared.MouseResult{Consumed: true}
	}
	if action != tview.MouseLeftDown && action != tview.MouseLeftClick {
		return shared.MouseResult{}
	}
	r := l.length
	if action == tview.MouseLeftDown {
		if !r.contains(x, y) {
			return shared.MouseResult{}
		}
		s.selected = focusLength
		s.draggingSlider = true
		s.setLengthFromPointer(r, 0, 0, x)
		return shared.MouseResult{Consumed: true, Capture: true}
	}
	click := func(r rect, f focus) bool {
		if !r.contains(x, y) {
			return false
		}
		s.selected = f
		s.activate()
		return true
	}
	if click(l.upper, focusUpper) || click(l.lower, focusLower) || click(l.numbers, focusNumbers) || click(l.symbols, focusSymbols) || click(l.ambig, focusAmbiguous) || click(l.store, focusStore) || (!l.compact && click(l.regen, focusRegen)) || click(l.copy, focusCopy) {
		return shared.MouseResult{Consumed: true, Capture: true}
	}
	if r.contains(x, y) {
		s.selected = focusLength
		s.setLengthFromPointer(r, 0, 0, x)
		return shared.MouseResult{Consumed: true, Capture: true}
	}
	return shared.MouseResult{}
}

func sliderRail(r rect) (start, width int) { return r.x + 3, max(8, r.w-8) }

func (s *View) setLengthFromPointer(r rect, ox, oy, pointerX int) {
	r.x += ox
	r.y += oy
	railStart, railWidth := sliderRail(r)
	pointerX = max(railStart, min(railStart+railWidth-1, pointerX))
	n := generator.MinLength + (pointerX-railStart)*(generator.MaxLength-generator.MinLength)/(railWidth-1)
	s.setLength(n)
}
