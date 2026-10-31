package tui

// runPrompted releases the terminal for subprocess pinentry and restores it
// afterward. Before the application starts, execution falls back to a direct call.
func (s *screen) runPrompted(fn func() error) error {
	if s.suspend == nil {
		return fn()
	}
	var err error
	if !s.suspend(func() { err = fn() }) {
		return fn()
	}
	return err
}
