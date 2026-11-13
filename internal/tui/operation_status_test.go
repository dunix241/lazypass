package tui

import (
	"context"
	"lazypass/internal/config"
	"lazypass/internal/tui/shared"
	"testing"
)

func TestOperationCompletionClearsOnlyItsOwnStatus(t *testing.T) {
	s := NewApp(config.Defaults(), "", nil).screen
	s.statuses.SetSlots(3)
	var completions []func(error)
	progress, run := s.statusOperationRunner(func(_ func(context.Context) error, done func(error)) { completions = append(completions, done) }, func() bool { return true })
	progress("Renaming…")
	run(func(context.Context) error { return nil }, func(error) {})
	progress("Copying…")
	run(func(context.Context) error { return nil }, func(error) {})
	s.notify(shared.Error, "Unrelated error")
	if len(s.statuses.Visible()) != 2 {
		t.Fatal("notification cleared active operation")
	}
	completions[0](nil)
	if len(s.statuses.Visible()) != 1 || s.statuses.Visible()[0].Text != "Copying…" {
		t.Fatal("completion cleared newer operation")
	}
	completions[1](nil)
	if len(s.statuses.Visible()) != 0 {
		t.Fatal("completed status remains")
	}
}

func TestOldCompletionCannotClearStatusAfterVaultSwitch(t *testing.T) {
	s := NewApp(config.Defaults(), "", nil).screen
	var finish func(error)
	progress, run := s.statusOperationRunner(func(_ func(context.Context) error, done func(error)) { finish = done }, func() bool { return true })
	progress("Old operation")
	run(func(context.Context) error { return nil }, func(error) {})
	s.clearOperationStatuses()
	progress("New operation")
	finish(nil)
	if s.statuses.Current() == nil || s.statuses.Current().Text != "New operation" {
		t.Fatal("stale completion removed current status")
	}
}
