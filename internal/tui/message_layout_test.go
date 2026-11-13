package tui

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/config"
	"lazypass/internal/tui/notification"
	"testing"
	"time"
)

func TestMessagePositionsAndSlotsSurviveResize(t *testing.T) {
	positions := []string{"topRight", "topLeft", "bottomRight", "bottomLeft"}
	for _, position := range positions {
		for _, slots := range []int{1, 2, 3} {
			for _, compact := range []bool{false, true} {
				cfg := config.Defaults()
				cfg.Compact = compact
				cfg.Messages = config.Messages{Notifications: config.MessageChannel{Position: position, Slots: slots}, Status: config.MessageChannel{Position: position, Slots: slots}}
				s := NewApp(cfg, "", nil).screen
				s.notifications.SetSlots(slots)
				s.statuses.SetSlots(slots)
				for i := 0; i < slots; i++ {
					s.notifications.Show(notification.Message{Text: "Notification fixture", Duration: time.Minute}, time.Now())
					s.statuses.Show(notification.Message{Text: "Status fixture", Duration: -1}, time.Now())
				}
				sim := tcell.NewSimulationScreen("UTF-8")
				if err := sim.Init(); err != nil {
					t.Fatal(err)
				}
				for _, size := range [][2]int{{120, 36}, {50, 24}, {32, 18}, {84, 24}} {
					sim.SetSize(size[0], size[1])
					s.SetRect(0, 0, size[0], size[1])
					s.Draw(sim)
					if len(s.notifications.Visible()) != slots || len(s.statuses.Visible()) != slots {
						t.Fatalf("%s slots=%d lost messages on resize", position, slots)
					}
					layout := s.contentLayout(size[0], size[1])
					footerY := size[1] - len(s.footerRows(size[0]))
					if layout.card.Y+layout.card.H > footerY {
						t.Fatal("message layout overlaps footer")
					}
				}
				sim.Fini()
			}
		}
	}
}
