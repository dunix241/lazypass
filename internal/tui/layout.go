package tui

import "lazypass/internal/tui/shared"

type layout struct {
	tooSmall, compact, short bool
	card                     shared.Rect
}

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
	compact, short := width < compactWidth, height < 20
	cardW := min(72, width-2)
	if compact {
		cardW = width - 2
	}
	cardH := cardHeight(compact, short)
	cardX := (width - cardW) / 2
	cardY := 1 + max(0, (height-2-cardH)/2)
	logo := calculateLogoLayout(width, height)
	cardY = max(cardY, logo.y+logo.h)
	return layout{compact: compact, short: short, card: shared.Rect{X: cardX, Y: cardY, W: cardW, H: cardH}}
}
