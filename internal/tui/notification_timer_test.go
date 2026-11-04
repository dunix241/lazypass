package tui

import (
	"context"
	"testing"
	"time"

	"lazypass/internal/tui/notification"
)

func TestNotificationTimerOnlyDispatchesAtExpiry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan func(), 2)
	s := &screen{dispatch: func(_ context.Context, fn func()) { updates <- fn }}
	defer s.stopNotificationTimer()
	s.scheduleNotification(ctx)
	if s.notificationTimer != nil {
		t.Fatal("idle notifications scheduled a timer")
	}
	s.notifications.Show(notification.Message{Text: "fixture", Duration: 20 * time.Millisecond}, time.Now())
	s.scheduleNotification(ctx)
	timer := s.notificationTimer
	s.scheduleNotification(ctx)
	if timer != s.notificationTimer {
		t.Fatal("unchanged draw rescheduled timer")
	}
	select {
	case apply := <-updates:
		apply()
	case <-time.After(time.Second):
		t.Fatal("expiry not dispatched")
	}
	if s.notifications.Current() != nil {
		t.Fatal("expired message retained")
	}
	if !s.notificationDeadline.IsZero() || s.notificationTimer != nil {
		t.Fatal("completed expiry retained scheduling state")
	}
	s.scheduleNotification(ctx)
	if s.notificationTimer != nil {
		t.Fatal("empty manager retained timer")
	}
}

func TestStaleNotificationCannotActivateNextAction(t *testing.T) {
	s := &screen{}
	called := false
	now := time.Now()
	s.notifications.Show(notification.Message{Text: "first", Actions: []notification.Action{{Run: func() { t.Fatal("stale action ran") }}}}, now)
	s.drawnNotification = s.notifications.Current()
	s.notifications.Show(notification.Message{Text: "next", Actions: []notification.Action{{Run: func() { called = true }}}}, now)
	s.notifications.Dismiss(now)
	if s.activateNotification(0) || called {
		t.Fatal("stale hit target activated queued action")
	}
}
