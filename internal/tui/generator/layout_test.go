package generator

import "testing"

func TestControlLayoutResponsive(t *testing.T) {
	for _, compact := range []bool{false, true} {
		for _, short := range []bool{false, true} {
			f := testFrame(compact, short)
			l := calculateLayout(f)
			if compact {
				if l.upper.x != l.lower.x || l.upper.y >= l.lower.y {
					t.Fatal("compact controls must stack")
				}
			} else {
				if l.upper.y != l.lower.y || l.upper.x >= l.lower.x {
					t.Fatal("wide controls must use two columns")
				}
				if short && (l.length.y-l.card.y != 6 || l.upper.y-l.length.y != 3) {
					t.Fatal("short-wide length spacing changed")
				}
			}
			for _, r := range []rect{l.length, l.upper, l.lower, l.numbers, l.symbols, l.ambig, l.store, l.copy} {
				if !f.Card.Contains(r.x, r.y) || !f.Card.Contains(r.x+r.w-1, r.y+r.h-1) {
					t.Fatal("control outside card")
				}
			}
		}
	}
}
