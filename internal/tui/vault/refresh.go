package vault

import (
	"context"
	"errors"

	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
)

func (s *View) refreshProvider() {
	if s.refreshing {
		return
	}
	s.ClearAuthenticators()
	s.refreshing = true
	s.syncing = true
	s.progress("Refreshing…")
	provider := s.vault
	s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error {
		if provider.Provider != nil && provider.Provider.Capabilities().Sync {
			_, err := provider.Sync(ctx)
			return err
		}
		return nil
	}, func(err error) {
		s.refreshing = false
		s.syncing = false
		if err != nil {
			var recovery vault.RecoveryCommand
			if s.deps.NotifyRecovery != nil && errors.As(err, &recovery) {
				s.deps.NotifyRecovery(recovery.UserMessage(), recovery.RecoveryCommand())
				return
			}
			var diagnostic vault.Diagnostic
			if errors.As(err, &diagnostic) {
				s.notify(shared.Warning, diagnostic.UserMessage())
				return
			}
			s.notify(shared.Error, "Unable to refresh vault")
			return
		}
		s.Reconcile("Vault refreshed")
	})
}
