package vault

import (
	"testing"

	"lazypass/internal/tui/shared"
)

func TestNotesHeightFollowsContentWithoutFocusExpansion(t *testing.T) {
	panel := shared.Rect{W: 52, H: 16}
	s := &StoreForm{}
	for _, tc := range []struct {
		text   string
		height int
	}{
		{"", 1},
		{"one", 1},
		{"one\ntwo", 2},
		{"one\ntwo\n", 3},
	} {
		s.notes = tc.text
		for _, focus := range []int{0, 3} {
			s.field = focus
			found := false
			for _, row := range s.fieldRows(panel) {
				if row.index == 3 {
					found = true
					if row.height != tc.height {
						t.Fatalf("notes height = %d, want %d", row.height, tc.height)
					}
				}
			}
			if !found {
				t.Fatal("notes missing")
			}
		}
	}
}
