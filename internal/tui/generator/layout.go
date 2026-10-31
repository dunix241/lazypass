package generator

import "lazypass/internal/tui/shared"

type rect struct{ x, y, w, h int }

func (r rect) shared() shared.Rect    { return shared.Rect{X: r.x, Y: r.y, W: r.w, H: r.h} }
func (r rect) contains(x, y int) bool { return r.shared().Contains(x, y) }

type layout struct {
	compact, short                                                          bool
	card, length, upper, lower, numbers, symbols, ambig, store, regen, copy rect
}

func calculateLayout(frame shared.Frame) layout {
	r := frame.Card
	cardX, cardY, cardW := r.X, r.Y, r.W
	l := layout{compact: frame.Compact, short: frame.Short, card: rect{r.X, r.Y, r.W, r.H}}
	if l.short && !l.compact {
		l.store = rect{cardX + cardW - 29, cardY + 2, 5, 1}
		l.regen = rect{cardX + cardW - 22, cardY + 2, 12, 1}
		l.copy = rect{cardX + cardW - 8, cardY + 2, 6, 1}
		l.length = rect{cardX + 2, cardY + 6, cardW - 4, 1}
		l.upper = rect{cardX + 2, cardY + 9, (cardW - 4) / 2, 1}
		l.lower = rect{cardX + cardW/2, cardY + 9, cardW/2 - 2, 1}
		l.numbers = rect{cardX + 2, cardY + 10, (cardW - 4) / 2, 1}
		l.symbols = rect{cardX + cardW/2, cardY + 10, cardW/2 - 2, 1}
		l.ambig = rect{cardX + 2, cardY + 11, cardW - 4, 1}
		return l
	}
	l.store = rect{cardX + cardW - 15, cardY + 3, 5, 1}
	if !l.compact {
		l.store = rect{cardX + cardW - 29, cardY + 3, 5, 1}
		l.regen = rect{cardX + cardW - 22, cardY + 3, 12, 1}
	}
	l.copy = rect{cardX + cardW - 8, cardY + 3, 6, 1}
	l.length = rect{cardX + 2, cardY + 7, cardW - 4, 1}
	if l.compact {
		l.upper = rect{cardX + 2, cardY + 10, cardW - 4, 1}
		l.lower = rect{cardX + 2, cardY + 11, cardW - 4, 1}
		l.numbers = rect{cardX + 2, cardY + 12, cardW - 4, 1}
		l.symbols = rect{cardX + 2, cardY + 13, cardW - 4, 1}
		l.ambig = rect{cardX + 2, cardY + 14, cardW - 4, 1}
		return l
	}
	l.upper = rect{cardX + 2, cardY + 10, (cardW - 4) / 2, 1}
	l.lower = rect{cardX + cardW/2, cardY + 10, cardW/2 - 2, 1}
	l.numbers = rect{cardX + 2, cardY + 11, (cardW - 4) / 2, 1}
	l.symbols = rect{cardX + cardW/2, cardY + 11, cardW/2 - 2, 1}
	l.ambig = rect{cardX + 2, cardY + 12, cardW - 4, 1}
	return l
}
