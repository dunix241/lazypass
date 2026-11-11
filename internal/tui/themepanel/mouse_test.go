package themepanel

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"lazypass/internal/tui/shared"
	"testing"
)

func TestThemeMouseFocusAndFooter(t *testing.T) {
	p, h := newPanel()
	p.Open()
	frame := shared.Frame{Viewport: shared.Rect{W: 84, H: 24}, Palette: h.colors}
	r := shared.CenteredPanel(frame.Viewport, 64, 15)
	p.HandleMouse(tview.MouseLeftClick, tcell.NewEventMouse(r.X+3, r.Y+3, tcell.Button1, 0), frame)
	if !p.IsOpen() {
		t.Fatal("single click applied theme")
	}
	footer := shared.Footer{Actions: p.footerActions()}
	clicked := false
	for x := r.X + 2; x < r.X+r.W-2; x++ {
		item, ok := footer.HitAction(shared.Rect{X: r.X + 2, Y: r.Y + r.H - 2, W: r.W - 4, H: 1}, shared.FooterOptions{MaxRows: 1}, x, r.Y+r.H-2)
		if ok && item.Hint == "/" {
			p.HandleMouse(tview.MouseLeftClick, tcell.NewEventMouse(x, r.Y+r.H-2, tcell.Button1, 0), frame)
			clicked = true
			break
		}
	}
	if !clicked || !p.filtering {
		t.Fatal("filter footer not clickable")
	}
}
