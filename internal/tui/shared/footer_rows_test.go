package shared

import (
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

func TestFooterRowsWrapAndPrioritize(t *testing.T) {
	f := Footer{Actions: []Action{
		{Hint: "a", Label: "optional", Priority: Optional},
		{Hint: "d", Label: "delete", Priority: Primary},
		{Hint: "q", Label: "quit", Priority: Essential},
	}}
	rows := f.Rows(12, FooterOptions{MaxRows: 2})
	if len(rows) != 2 || strings.Contains(strings.Join(rows, " "), "optional") || !strings.Contains(strings.Join(rows, " "), "q quit") {
		t.Fatal("footer priority failed")
	}
	for _, row := range rows {
		if uniseg.StringWidth(row) > 12 {
			t.Fatal("footer exceeds width")
		}
	}
}

func TestFooterHitPreservesDuplicateLabelIdentity(t *testing.T) {
	clicked := 0
	f := Footer{Actions: []Action{
		{Label: "same", OnClick: func() { clicked = 1 }},
		{Label: "same", OnClick: func() { clicked = 2 }},
	}}
	bounds := Rect{X: 5, Y: 3, W: 20, H: 2}
	// "same  •  same" occupies 13 cells, centered at x=8.
	action, ok := f.HitAction(bounds, FooterOptions{MaxRows: 2}, 18, 3)
	if !ok {
		t.Fatal("second duplicate action not found")
	}
	action.OnClick()
	if clicked != 2 {
		t.Fatal("duplicate label dispatched the wrong action")
	}
	if _, ok := f.HitAction(bounds, FooterOptions{MaxRows: 2}, 13, 3); ok {
		t.Fatal("separator is clickable")
	}
	if _, ok := f.HitAction(bounds, FooterOptions{MaxRows: 2}, 18, 5); ok {
		t.Fatal("out-of-bounds row is clickable")
	}
}

func TestFooterHitLabelContainingSeparator(t *testing.T) {
	f := Footer{Actions: []Action{{Label: "one  •  two"}}}
	action, ok := f.HitAction(Rect{W: 20, H: 1}, FooterOptions{MaxRows: 1}, 12, 0)
	if !ok || action.Label != "one  •  two" {
		t.Fatal("label was split into fake actions")
	}
}
