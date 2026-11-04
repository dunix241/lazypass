package tui

import (
	"context"
	"lazypass/internal/debug"
	"time"
)

// Called on the UI thread after drawing; at most one expiry is scheduled.
func (s *screen) scheduleNotification(ctx context.Context) {
	deadline := s.notifications.Deadline()
	if deadline.Equal(s.notificationDeadline) {
		return
	}
	s.stopNotificationTimer()
	if deadline.IsZero() || ctx.Err() != nil {
		return
	}
	s.notificationDeadline = deadline
	s.notificationTimer = time.AfterFunc(max(0, time.Until(deadline)), func() {
		if ctx.Err() != nil {
			return
		}
		s.dispatch(ctx, func() {
			if !s.notificationDeadline.Equal(deadline) {
				return
			}
			s.notificationTimer = nil
			s.notificationDeadline = time.Time{}
			message := s.notifications.Current()
			if s.notifications.Tick(time.Now()) && message != nil && message.ID == "update" {
				debug.Event("update notification expired")
			}
		})
	})
}

func (s *screen) stopNotificationTimer() {
	if s.notificationTimer != nil {
		s.notificationTimer.Stop()
		s.notificationTimer = nil
	}
	s.notificationDeadline = time.Time{}
}
