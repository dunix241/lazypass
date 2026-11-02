package tui

import (
	"context"
	"testing"
	"time"

	"lazypass/internal/vault"
)

func TestSharedOperationRunnerYieldsAndCompletesOnDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	updates := make(chan func(), 1)
	s := &screen{running: true, operationContext: ctx, dispatch: func(_ context.Context, fn func()) { updates <- fn }}
	started, release := make(chan struct{}), make(chan struct{})
	completed := false
	s.operationRunner()(func(ctx context.Context) error {
		if !vault.IsNoninteractive(ctx) {
			t.Error("background operation allowed prompt")
		}
		close(started)
		<-release
		return nil
	}, func(err error) {
		if err != nil {
			t.Error(err)
		}
		completed = true
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker not started")
	}
	if completed {
		t.Fatal("operation completed before release")
	}
	close(release)
	select {
	case update := <-updates:
		if completed {
			t.Fatal("completion ran outside dispatch")
		}
		update()
	case <-time.After(time.Second):
		t.Fatal("completion not dispatched")
	}
	if !completed {
		t.Fatal("completion missing")
	}
}

func TestSharedOperationCancellationSkipsCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	s := &screen{running: true, operationContext: ctx, dispatch: func(context.Context, func()) { t.Error("canceled operation dispatched") }}
	s.operationRunner()(func(ctx context.Context) error { <-ctx.Done(); close(finished); return ctx.Err() }, func(error) { t.Error("canceled completion ran") })
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("operation not canceled")
	}
}
