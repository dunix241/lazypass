package tui

import "testing"

func TestCalculateLayoutWideCentersBoundedCard(t *testing.T) {
	l := calculateLayout(120, 40)
	if l.tooSmall || l.compact {
		t.Fatalf("120x40 should use wide layout: %+v", l)
	}
	if l.card.w != 72 || l.card.x != 24 {
		t.Fatalf("unexpected card bounds: %+v", l.card)
	}
	if l.upper.y != l.lower.y || l.upper.x >= l.lower.x {
		t.Fatalf("wide toggles should use two columns: upper=%+v lower=%+v", l.upper, l.lower)
	}
}

func TestCalculateLayoutCompactUsesSingleColumn(t *testing.T) {
	l := calculateLayout(50, 18)
	if l.tooSmall || !l.compact {
		t.Fatalf("50x18 should use compact layout: %+v", l)
	}
	if l.upper.x != l.lower.x || l.upper.y >= l.lower.y {
		t.Fatalf("compact toggles should be stacked: upper=%+v lower=%+v", l.upper, l.lower)
	}
}

func TestCalculateLayoutShortWideKeepsTwoColumns(t *testing.T) {
	l := calculateLayout(84, 18)
	if l.tooSmall || l.compact || !l.short {
		t.Fatalf("84x18 should use short wide layout: %+v", l)
	}
	if l.upper.y != l.lower.y || l.upper.x >= l.lower.x {
		t.Fatalf("short wide layout should retain two columns: upper=%+v lower=%+v", l.upper, l.lower)
	}
	if l.length.y-l.card.y != 6 || l.upper.y-l.length.y != 3 {
		t.Fatalf("short wide layout should leave space around length: card=%+v length=%+v upper=%+v", l.card, l.length, l.upper)
	}
	if l.card.y+l.card.h > 17 { // Row 17 is reserved for the footer.
		t.Fatalf("card should fit above footer: %+v", l.card)
	}
}

func TestCalculateLayoutMediumWidthKeepsTwoColumns(t *testing.T) {
	l := calculateLayout(60, 17)
	if l.tooSmall || l.compact {
		t.Fatalf("60x17 should use two columns: %+v", l)
	}
	if l.upper.y != l.lower.y || l.upper.x >= l.lower.x {
		t.Fatalf("medium width should retain two columns: upper=%+v lower=%+v", l.upper, l.lower)
	}
}

func TestCardStartsBelowLogo(t *testing.T) {
	for _, size := range [][2]int{{84, 24}, {64, 20}, {50, 18}} {
		logo := calculateLogoLayout(size[0], size[1])
		card := calculateLayout(size[0], size[1]).card
		if card.y < logo.y+logo.h {
			t.Fatalf("%dx%d card overlaps logo: card=%+v logo=%+v", size[0], size[1], card, logo)
		}
	}
}

func TestCalculateLayoutRejectsTinyViewport(t *testing.T) {
	for _, size := range [][2]int{{31, 20}, {80, 9}, {32, 17}, {50, 17}} {
		if !calculateLayout(size[0], size[1]).tooSmall {
			t.Fatalf("%dx%d should be too small", size[0], size[1])
		}
	}
}
