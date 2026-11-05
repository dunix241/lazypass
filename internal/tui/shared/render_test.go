package shared

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
)

func TestTruncatePreservesGraphemesAndCellWidth(t *testing.T) {
	for _, value := range []string{"plain text", "🙂🙂🙂🙂", "e\u0301e\u0301e\u0301", "👨‍👩‍👧‍👦 family"} {
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

func TestPrintStyledPreservesGraphemesAndStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(30, 3)
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlue).Bold(true)
	PrintStyled(screen, 1, 1, "🙂e\u0301[x]", style)
	for _, cell := range []struct {
		x    int
		text string
	}{
		{1, "🙂"}, {3, "e\u0301"}, {4, "["}, {5, "x"}, {6, "]"},
	} {
		text, gotStyle, _ := screen.Get(cell.x, 1)
		if text != cell.text || gotStyle != style {
			t.Fatalf("cell %d: got %q %v, want %q %v", cell.x, text, gotStyle, cell.text, style)
		}
	}
}
