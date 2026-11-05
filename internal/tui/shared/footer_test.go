package shared

import (
	"testing"

	"github.com/rivo/uniseg"
)

func TestHintsFitTerminalCells(t *testing.T) {
	for width := -2; width < 20; width++ {
		for _, options := range [][]string{nil, {"🙂🙂 wide hint", "🙂 hint"}, {"long fallback"}} {
			got := FitHints(width, options...)
			if uniseg.StringWidth(got) > max(0, width) {
				t.Fatalf("hint %q exceeds %d cells", got, width)
			}
		}
	}
}
