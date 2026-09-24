package tui

import "github.com/gdamore/tcell/v2"

type logoStyle uint8

const (
	logoOff logoStyle = iota
	logoWordmark
)

type logoLayout struct {
	style      logoStyle
	x, y, w, h int
}

func calculateLogoLayout(width, height int) logoLayout {
	if width < 32 || height < 15 || (width < 64 && height < 16) {
		return logoLayout{}
	}
	if logoFits(width, height, 1) {
		const wordmarkWidth = 8
		return logoLayout{style: logoWordmark, x: (width - wordmarkWidth) / 2, w: wordmarkWidth, h: 1}
	}
	return logoLayout{}
}

func logoFits(width, height, logoH int) bool {
	cardH := cardHeight(width < compactWidth, height < 20)
	cardY := 1 + max(0, (height-2-cardH)/2)
	if logoH > cardY {
		cardY = logoH
	}
	return cardY+cardH <= height-1
}

func drawLogo(screen tcell.Screen, logo logoLayout, ox, oy int) {
	if logo.style == logoWordmark {
		printAt(screen, ox+logo.x, oy+logo.y, "lazypass", theme.accent)
	}
}

func (l logoLayout) right() int { return l.x + l.w }
