package tui

type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

type layout struct {
	tooSmall bool
	compact  bool
	short    bool
	card     rect
	length   rect
	upper    rect
	lower    rect
	numbers  rect
	symbols  rect
	ambig    rect
	store    rect
	regen    rect
	copy     rect
}

// compactWidth is the minimum width for two toggle columns.
// One column needs 21 cells ("[✓] Lowercase [L]") plus card margins.
const compactWidth = 56

func cardHeight(compact, short bool) int {
	if short && !compact {
		return 13
	}
	if compact {
		return 16
	}
	return 17
}

func calculateLayout(width, height int) layout {
	if width < 32 || height < 15 || (width < compactWidth && height < 18) {
		return layout{tooSmall: true}
	}
	compact := width < compactWidth
	short := height < 20
	cardW := min(72, width-2)
	if compact {
		cardW = width - 2
	}
	cardH := cardHeight(compact, short)
	cardX := (width - cardW) / 2
	cardY := 1 + max(0, (height-2-cardH)/2)
	logo := calculateLogoLayout(width, height)
	cardY = max(cardY, logo.y+logo.h)
	l := layout{compact: compact, short: short, card: rect{cardX, cardY, cardW, cardH}}
	if short && !compact {
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
	if !compact {
		l.store = rect{cardX + cardW - 29, cardY + 3, 5, 1}
		l.regen = rect{cardX + cardW - 22, cardY + 3, 12, 1}
	}
	l.copy = rect{cardX + cardW - 8, cardY + 3, 6, 1}
	l.length = rect{cardX + 2, cardY + 7, cardW - 4, 1}
	if compact {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
