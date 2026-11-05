package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	"lazypass/internal/config"
	"lazypass/internal/update"
)

func TestUpdateTaskModes(t *testing.T) {
	for _, tc := range []struct {
		name                                  string
		mode                                  config.UpdateMode
		available, eligible                   bool
		installErr                            error
		wantChecks, wantInstalls, wantResults int
	}{
		{"off", config.UpdatesOff, true, true, nil, 0, 0, 0},
		{"notify", config.UpdatesNotify, true, true, nil, 1, 0, 1},
		{"current", config.UpdatesAuto, false, true, nil, 1, 0, 0},
		{"auto", config.UpdatesAuto, true, true, nil, 1, 1, 1},
		{"manual fallback", config.UpdatesAuto, true, false, nil, 1, 1, 1},
		{"install failure", config.UpdatesAuto, true, true, errors.New("fixture"), 1, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checks, installs, results := 0, 0, 0
			runUpdateTask(context.Background(), tc.mode, func(context.Context) (update.Release, bool, error) {
				checks++
				return update.Release{Version: "v1.2.3"}, tc.available, nil
			}, func(context.Context, string) (bool, error) {
				installs++
				return tc.eligible, tc.installErr
			}, func(result updateResult) {
				results++
				if result.installed != (tc.eligible && tc.mode == config.UpdatesAuto && tc.installErr == nil) || !errors.Is(result.err, tc.installErr) {
					t.Fatalf("unexpected outcome: %+v", result)
				}
			})
			if checks != tc.wantChecks || installs != tc.wantInstalls || results != tc.wantResults {
				t.Fatalf("calls: check=%d install=%d emit=%d", checks, installs, results)
			}
		})
	}
}

func TestUpdateTaskCancellationDuringInstall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, done := make(chan struct{}), make(chan struct{})
	results := make(chan updateResult, 1)
	go func() {
		defer close(done)
		runUpdateTask(ctx, config.UpdatesAuto, func(context.Context) (update.Release, bool, error) {
			return update.Release{Version: "v1.2.3"}, true, nil
		}, func(ctx context.Context, _ string) (bool, error) {
			close(started)
			<-ctx.Done()
			return true, ctx.Err()
		}, func(result updateResult) { results <- result })
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not cancel")
	}
	select {
	case <-results:
		t.Fatal("cancelled worker dispatched UI work")
	default:
	}
}
