package tui

import (
	"context"
	"fmt"
	"time"

	"lazypass/internal/tui/notification"
	"lazypass/internal/tui/shared"
	vaultui "lazypass/internal/tui/vault"
)

// Progress is emitted just before an operation is submitted. Bind that status
// to the submission, not to whichever notification happens to arrive next.
func (s *screen) statusOperationRunner(run vaultui.OperationRunner, active func() bool) (func(string), vaultui.OperationRunner) {
	var pending string
	progress := func(text string) {
		if !active() {
			return
		}
		if pending == "" {
			s.statusSequence++
			pending = fmt.Sprintf("operation:%d", s.statusSequence)
		}
		s.statuses.Show(notification.Message{ID: pending, Text: text, Level: shared.Info, Duration: -1}, time.Now())
	}
	runner := func(work func(context.Context) error, done func(error)) {
		id := pending
		pending = ""
		run(work, func(err error) {
			if id != "" {
				s.statuses.Remove(id, time.Now())
			}
			done(err)
		})
	}
	return progress, runner
}

func (s *screen) clearOperationStatuses() {
	// Include queued statuses, which may exceed the configured visible slots.
	s.statuses = notification.Manager{}
	s.statuses.SetSlots(s.cfg.Messages.Resolved().Status.Slots)
}
