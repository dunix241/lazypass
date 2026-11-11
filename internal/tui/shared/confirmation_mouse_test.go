package shared

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"testing"
)

func TestConfirmationMouseUsesVisibleActions(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		confirmed, canceled := false, false
		c := NewConfirmation(ConfirmationOptions{Message: "Delete fixture?", OnConfirm: func() { confirmed = true }, OnCancel: func() { canceled = true }})
		frame := Frame{Viewport: Rect{W: 80, H: 24}, Palette: DefaultPalette()}
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		c.Draw(sim, frame)
		text := "y yes"
		if cancel {
			text = "<esc> cancel"
		}
		clicked := false
		for y := 0; y < 24 && !clicked; y++ {
			for x := 0; x < 80-len(text); x++ {
				match := true
				for i, ch := range text {
					cell, _, _ := sim.Get(x+i, y)
					if cell != string(ch) {
						match = false
						break
					}
				}
				if match {
					c.HandleMouse(tview.MouseLeftClick, tcell.NewEventMouse(x, y, tcell.Button1, 0), frame)
					clicked = true
					break
				}
			}
		}
		sim.Fini()
		if !clicked || confirmed == cancel || canceled != cancel {
			t.Fatal("visible confirmation action not clickable")
		}
	}
}
