package tui

import (
	"context"
	"time"

	"lazypass/internal/build"
	"lazypass/internal/config"
	"lazypass/internal/debug"
	"lazypass/internal/install"
	"lazypass/internal/state"
	"lazypass/internal/task"
	"lazypass/internal/tui/notification"
	"lazypass/internal/tui/shared"
	"lazypass/internal/update"
)

const updateStartupDelay = 3 * time.Second

func (s *screen) updateTask() (task.Task, bool) {
	mode := s.cfg.Updates
	if mode == config.UpdatesOff {
		debug.Event("update skipped disabled")
		return task.Task{}, false
	}
	if !update.Stable(build.Version) {
		debug.Event("update skipped development")
		return task.Task{}, false
	}
	debug.Event("update scheduled")
	checker := update.Checker{Source: update.GitHub{}, Store: state.UpdateStore{}, IncludeDismissed: mode == config.UpdatesAuto}
	return task.Task{ID: "update check", Delay: updateStartupDelay, Run: func(ctx context.Context) error {
		runUpdateTask(ctx, mode, func(ctx context.Context) (update.Release, bool, error) {
			return checker.Check(ctx, build.Version, false)
		}, install.Automatic, func(result updateResult) {
			s.dispatch(ctx, func() { s.showUpdateResult(result) })
		})
		return nil
	}}, true
}

type updateResult struct {
	release   update.Release
	installed bool
	err       error
}

// Called only by the background worker; emit is the sole UI handoff.
func runUpdateTask(ctx context.Context, mode config.UpdateMode,
	check func(context.Context) (update.Release, bool, error),
	install func(context.Context, string) (bool, error), emit func(updateResult)) {
	if mode == config.UpdatesOff || ctx.Err() != nil {
		return
	}
	debug.Event("update check started")
	release, available, err := check(ctx)
	if err != nil {
		if ctx.Err() == nil {
			debug.Failure("update check", debug.Unavailable)
		}
		return
	}
	if !available || ctx.Err() != nil {
		return
	}
	result := updateResult{release: release}
	if mode == config.UpdatesAuto {
		debug.Event("update automatic install started")
		attempted, err := install(ctx, release.Version)
		result.installed, result.err = attempted && err == nil, err
		if err != nil && ctx.Err() == nil {
			debug.Failure("update install", debug.Unavailable)
		}
	}
	if ctx.Err() == nil {
		emit(result)
	}
}

func (s *screen) showUpdateResult(result updateResult) {
	switch {
	case result.installed:
		s.notifications.Show(notification.Message{
			ID: "update", Text: result.release.Version + " installed; restart lazypass to use it", Level: shared.Success, Duration: -1,
			Actions: []notification.Action{{Label: "dismiss"}},
		}, time.Now())
	case result.err != nil:
		s.notifications.Show(notification.Message{
			ID: "update", Text: "Automatic update failed; run lazypass update to retry", Level: shared.Warning, Duration: 8 * time.Second,
		}, time.Now())
	default:
		s.showUpdate(result.release)
	}
}

func (s *screen) showUpdate(release update.Release) {
	s.notifications.Show(notification.Message{
		ID: "update", Text: release.Version + " available", Level: shared.Info, Duration: 8 * time.Second,
		Actions: []notification.Action{{Label: "dismiss", Run: func() {
			if s.tasks == nil {
				return
			}
			if _, err := s.tasks.Submit(task.Task{ID: "update dismiss", Run: func(context.Context) error {
				return (update.Checker{Store: state.UpdateStore{}}).Dismiss(release)
			}}); err != nil {
				debug.Failure("update dismiss schedule", debug.Unavailable)
			}
		}}},
	}, time.Now())
}
