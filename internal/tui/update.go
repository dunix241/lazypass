package tui

import (
	"context"
	"time"

	"lazypass/internal/build"
	"lazypass/internal/debug"
	"lazypass/internal/state"
	"lazypass/internal/tui/notification"
	"lazypass/internal/tui/shared"
	"lazypass/internal/update"
)

const updateStartupDelay = 3 * time.Second

func (s *screen) checkUpdates(ctx context.Context) {
	if !s.cfg.Updates.Check {
		debug.Event("update skipped disabled")
		return
	}
	if !update.Stable(build.Version) {
		debug.Event("update skipped development")
		return
	}
	debug.Event("update scheduled")
	checker := update.Checker{Source: update.GitHub{}, Store: state.UpdateStore{}}
	go func() {
		timer := time.NewTimer(updateStartupDelay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			debug.Event("update canceled before check")
			return
		case <-timer.C:
		}
		debug.Event("update check started")
		release, available, err := checker.Check(ctx, build.Version, false)
		if err != nil {
			if ctx.Err() == nil {
				debug.Failure("update check", debug.Unavailable)
			}
			return
		}
		if available {
			debug.Event("update available dispatching")
			s.dispatch(ctx, func() { s.showUpdate(release); debug.Event("update notification queued") })
		} else {
			debug.Event("update no notification")
		}
	}()
}

func (s *screen) showUpdate(release update.Release) {
	s.notifications.Show(notification.Message{
		ID: "update", Text: release.Version + " available", Level: shared.Info, Duration: 8 * time.Second,
		Actions: []notification.Action{{Label: "dismiss", Run: func() {
			go func() {
				if err := (update.Checker{Store: state.UpdateStore{}}).Dismiss(release); err != nil {
					debug.Failure("update dismiss", debug.IO)
				}
			}()
		}}},
	}, time.Now())
}
