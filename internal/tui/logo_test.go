package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/config"
)

func TestCalculateLogoLayout(t *testing.T) {
	tests := []struct {
		width, height int
		style         logoStyle
		heightRows    int
	}{
		{84, 24, logoWordmark, 1},
		{84, 18, logoWordmark, 1},
		{64, 20, logoWordmark, 1},
		{63, 20, logoWordmark, 1},
		{31, 20, logoOff, 0},
	}
	for _, test := range tests {
		logo := calculateLogoLayout(test.width, test.height)
		if logo.style != test.style || logo.h != test.heightRows {
			t.Errorf("%dx%d logo = %+v, want style %d height %d", test.width, test.height, logo, test.style, test.heightRows)
			continue
		}
		if logo.style != logoOff && logo.x != (test.width-logo.w)/2 {
			t.Errorf("%dx%d logo x = %d, want centered %d", test.width, test.height, logo.x, (test.width-logo.w)/2)
		}
	}
}

func TestLogoNeverOverlapsCardOrFooter(t *testing.T) {
	for _, size := range [][2]int{{84, 24}, {64, 20}, {50, 18}, {84, 18}} {
		width, height := size[0], size[1]
		a := NewApp(config.Defaults(), "", nil)
		if err := a.screen.regenerate(); err != nil {
			t.Fatal(err)
		}
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(width, height)
		a.screen.SetRect(0, 0, width, height)
		a.screen.Draw(sim)
		defer sim.Fini()

		logo := calculateLogoLayout(width, height)
		card := calculateLayout(width, height).card
		if card.y < logo.y+logo.h {
			t.Fatalf("%dx%d card overlaps logo: card=%+v logo=%+v", width, height, card, logo)
		}
		if logo.style == logoWordmark {
			wordmark, _, _ := sim.Get(logo.x, logo.y)
			if wordmark != "l" {
				t.Fatalf("%dx%d logo wordmark missing, got %q", width, height, wordmark)
			}
		}
		var footerRow strings.Builder
		for xx := 0; xx < width; xx++ {
			cell, _, _ := sim.Get(xx, height-1)
			footerRow.WriteString(cell)
		}
		if !strings.Contains(footerRow.String(), "quit") {
			t.Fatalf("%dx%d footer was displaced by logo: %q", width, height, footerRow.String())
		}
	}
}
