package tui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/config"
	"lazypass/internal/update"
)

func TestUpdateNoticeUsesQueueAndExpires(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	s := a.screen
	s.notify(notificationSuccess, "Copied to clipboard")
	s.showUpdate(update.Release{Version: "v2.0.0"})
	if s.notifications.Pending() != 1 || s.notifications.Current().Text != "Copied to clipboard" {
		t.Fatal("update replaced operation notification")
	}
	s.notifications.Tick(time.Now().Add(4 * time.Second))
	if s.notifications.Current() == nil || s.notifications.Current().ID != "update" {
		t.Fatal("update did not advance from queue")
	}
	s.notifications.Tick(time.Now().Add(13 * time.Second))
	if s.notifications.Current() != nil {
		t.Fatal("update did not expire")
	}
}

func TestNotificationFitsCompactTerminal(t *testing.T) {
	for _, width := range []int{32, 50, 80, 120} {
		a := NewApp(config.Defaults(), "", nil)
		s := a.screen
		s.SetRect(0, 0, width, 24)
		s.showUpdate(update.Release{Version: "v2.0.0"})
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		sim.SetSize(width, 24)
		s.Draw(sim)
		if len(s.notificationBounds) == 0 {
			t.Fatalf("notification hidden at width %d", width)
		}
		if s.notificationBounds[0].X <= width/2 {
			t.Fatalf("notification rendered on the left at width %d", width)
		}
		sim.Fini()
	}
}
