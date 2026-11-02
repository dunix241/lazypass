package vault

import (
	"context"

	"lazypass/internal/tui/shared"
)

func (s *View) refreshProvider() {
	if s.refreshing {
		return
	}
	s.refreshing = true
	s.notify(shared.Info, "Refreshing…")
	provider := s.vault
	s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error {
		if provider.Provider != nil && provider.Provider.Capabilities().Sync {
			_, err := provider.Sync(ctx)
			return err
		}
		return nil
	}, func(err error) {
		s.refreshing = false
		if err != nil {
			s.notify(shared.Error, "Unable to refresh vault")
			return
		}
		s.Refresh()
		s.notify(shared.Success, "Vault refreshed")
	})
}
