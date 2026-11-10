package tui

import (
	"context"
	"errors"

	"lazypass/internal/tui/shared"
	vaultui "lazypass/internal/tui/vault"
	"lazypass/internal/vault"
)

func (s *screen) operationRunner(provider vault.Provider) vaultui.OperationRunner {
	_, hasPrompt := provider.(vault.PassphraseProvider)
	return func(work func(context.Context) error, done func(error)) {
		if !s.running {
			done(s.runPrompted(work))
			return
		}
		ctx := s.operationContext
		workContext := vault.WithoutPrompt(ctx)
		if hasPrompt {
			workContext = vault.WithPassphrasePrompt(ctx, func(request context.Context) (string, error) { return s.requestVaultPassphrase(request, provider) })
		}
		go func() {
			err := work(workContext)
			if ctx.Err() != nil {
				return
			}
			s.dispatch(ctx, func() {
				if ctx.Err() != nil {
					return
				}
				if errors.Is(err, vault.ErrPromptRequired) {
					if !s.isActiveProvider(provider) {
						done(context.Canceled)
						return
					}
					err = s.withTerminal(func() error { return work(ctx) })
				}
				done(err)
			})
		}()
	}
}

func (s *screen) isActiveProvider(provider vault.Provider) bool {
	return s.vaultIndex >= 0 && s.vaultIndex < len(s.vaults) && s.vaults[s.vaultIndex].Service.Provider == provider
}

func (s *screen) refreshVaultEarly() {
	s.vaultActivation++
	activation := s.vaultActivation
	view := s.vault
	provider := s.vaults[s.vaultIndex].Service.Provider
	unlocker, ok := provider.(interface{ UnlockEarly(context.Context) error })
	if !ok || !s.running {
		view.Refresh()
		return
	}
	view.BrowseCached()
	s.operationRunner(provider)(unlocker.UnlockEarly, func(err error) {
		if activation != s.vaultActivation || !s.isActiveProvider(provider) {
			return
		}
		if err == nil {
			view.Authenticated()
			return
		}
		var diagnostic vault.Diagnostic
		if errors.As(err, &diagnostic) {
			var recovery vault.RecoveryCommand
			if errors.As(err, &recovery) {
				s.notifyRecovery(diagnostic.UserMessage(), recovery.RecoveryCommand())
			} else {
				s.notify(shared.Error, diagnostic.UserMessage())
			}
		} else if !errors.Is(err, context.Canceled) {
			s.notify(shared.Warning, "Vault unlock canceled or failed")
		}
	})
}
