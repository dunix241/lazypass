package shared

import (
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

func TestTruncatePreservesGraphemesAndCellWidth(t *testing.T) {
	for _, value := range []string{"plain text", "世界您好", "e\u0301e\u0301e\u0301", "👨‍👩‍👧‍👦 family"} {
		for width := 0; width < 12; width++ {
			got := Truncate(value, width)
			if uniseg.StringWidth(got) > width {
				t.Fatalf("width exceeded: %q (%d)", got, width)
			}
			if strings.HasPrefix(got, "\u0301") {
				t.Fatal("split combining grapheme")
			}
		}
	}
}
