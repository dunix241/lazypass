package tui

import (
	"context"
	"errors"

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
