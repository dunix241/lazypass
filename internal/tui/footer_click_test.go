package tui

import (
	"testing"

	"lazypass/internal/config"
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestGeneratorFooterClickCopiesAndGlobalViewRoutes(t *testing.T) {
	copies := 0
	a := NewApp(config.Defaults(), "", func(string) error { copies++; return nil })
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.SetRect(0, 0, 84, 24)
	click := func(label string) {
		t.Helper()
		footer := a.screen.generator.FooterActions()
		rows := a.screen.generator.FooterRows(84, 2)
		bounds := shared.Rect{X: 1, Y: 24 - len(rows), W: 82, H: len(rows)}
		for y := bounds.Y; y < 24; y++ {
			for x := 1; x < 83; x++ {
				item, ok := footer.HitAction(bounds, shared.FooterOptions{MaxRows: 2}, x, y)
				if ok && item.Label == label {
					a.screen.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x, y, tcell.Button1, 0), nil)
					return
				}
			}
		}
		t.Fatal("footer action missing: " + label)
	}
	click("copy")
	if copies != 1 {
		t.Fatal("copy click not executed")
	}
	click("view")
	if a.CurrentRoute() != VaultRoute {
		t.Fatal("global footer click not routed")
	}
}
