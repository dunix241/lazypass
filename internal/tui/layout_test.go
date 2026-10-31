package tui

import "testing"

func TestCalculateLayoutResponsiveCard(t *testing.T) {
	for _, test := range []struct {
		w, h           int
		compact, short bool
		cardW, cardH   int
	}{{120, 40, false, false, 72, 17}, {50, 18, true, true, 48, 16}, {84, 18, false, true, 72, 13}, {60, 17, false, true, 58, 13}} {
		l := calculateLayout(test.w, test.h)
		if l.tooSmall || l.compact != test.compact || l.short != test.short || l.card.W != test.cardW || l.card.H != test.cardH || l.card.X != (test.w-test.cardW)/2 {
			t.Fatalf("%dx%d layout: %+v", test.w, test.h, l)
		}
		if l.card.Y+l.card.H > test.h-1 {
			t.Fatal("card overlaps footer")
		}
	}
}

func TestCardStartsBelowLogo(t *testing.T) {
	for _, size := range [][2]int{{84, 24}, {64, 20}, {50, 18}} {
		logo := calculateLogoLayout(size[0], size[1])
		card := calculateLayout(size[0], size[1]).card
		if card.Y < logo.y+logo.h {
			t.Fatal("card overlaps logo")
		}
	}
}

func TestCalculateLayoutRejectsTinyViewport(t *testing.T) {
	for _, size := range [][2]int{{31, 20}, {80, 9}, {32, 17}, {50, 17}} {
		if !calculateLayout(size[0], size[1]).tooSmall {
			t.Fatal("tiny viewport not rejected")
		}
	}
}
