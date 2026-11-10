package vault

import (
	"lazypass/internal/tui/shared"
)

// Reconcile reports success only after the active location's rows are applied.
// A newer navigation refresh inherits the completion rather than reporting early.
func (s *View) Reconcile(success string) {
	s.progress("Refreshing…")
	s.refreshCompletion = func(err error) {
		if err != nil {
			s.notify(shared.Warning, success+", but refresh failed; showing cached entries")
			return
		}
		s.notify(shared.Success, success)
	}
	s.Refresh()
}

func (s *View) progress(message string) {
	if s.deps.Progress != nil {
		s.deps.Progress(message)
	} else {
		s.notify(shared.Info, message)
	}
}

func (s *StoreForm) progress(message string) {
	if s.deps.Progress != nil {
		s.deps.Progress(message)
	} else {
		s.notify(shared.Info, message)
	}
}
