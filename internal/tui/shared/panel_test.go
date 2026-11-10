package shared

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestPanelLayoutAndActions(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(60, 20)
	r := CenteredPanel(Rect{W: 60, H: 20}, 40, 10)
	called := false
	actions := []Action{{Hint: "Enter", Label: "delete", Enabled: func() bool { return false }, Match: func(e *tcell.EventKey) bool { return e.Key() == tcell.KeyEnter }, Invoke: func() { called = true }}}
	Panel{Bounds: r, Title: "Delete", Border: tcell.ColorRed, Actions: actions}.Draw(screen, func(content Rect) {
		if content.X != r.X+2 || content.W != r.W-4 || content.H != r.H-4 {
			t.Fatalf("bad content bounds: %+v", content)
		}
	})
	if !HandleAction(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), actions) || called {
		t.Fatal("disabled action invoked")
	}
}
