package tui

import (
	"context"
	"errors"

	vaultui "lazypass/internal/tui/vault"
	"lazypass/internal/vault"
)

func (s *screen) operationRunner(providers ...vault.Provider) vaultui.OperationRunner {
	var provider vault.Provider
	if len(providers) > 0 {
		provider = providers[0]
	}
	_, hasPrompt := provider.(vault.PassphraseProvider)
	return func(work func(context.Context) error, done func(error)) {
		if !s.running {
			done(s.runPrompted(work))
			return
		}
		ctx := s.operationContext
		workContext := vault.WithoutPrompt(ctx)
		if hasPrompt {
			workContext = vault.WithPassphrasePrompt(ctx, s.requestPassphrase)
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
					err = s.withTerminal(func() error { return work(ctx) })
				}
				done(err)
			})
		}()
	}
}
