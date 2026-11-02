package tui

import (
	"context"
	"errors"

	vaultui "lazypass/internal/tui/vault"
	"lazypass/internal/vault"
)

func (s *screen) operationRunner() vaultui.OperationRunner {
	return func(work func(context.Context) error, done func(error)) {
		if !s.running {
			done(s.runPrompted(work))
			return
		}
		ctx := s.operationContext
		go func() {
			err := work(vault.WithoutPrompt(ctx))
			if ctx.Err() != nil {
				return
			}
			s.dispatch(ctx, func() {
				if errors.Is(err, vault.ErrPromptRequired) {
					err = s.withTerminal(func() error { return work(ctx) })
				}
				done(err)
			})
		}()
	}
}
