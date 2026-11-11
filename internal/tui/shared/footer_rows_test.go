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
