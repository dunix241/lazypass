package tui

import (
	"context"
	"errors"

	"lazypass/internal/vault"
)

// Try cached credentials with prompts disabled before releasing the terminal.
// Only a provider's explicit prompt-required result permits an interactive retry.
func (s *screen) runPrompted(fn func(context.Context) error) error {
	ctx := context.Background()
	err := fn(vault.WithoutPrompt(ctx))
	if !errors.Is(err, vault.ErrPromptRequired) {
		return err
	}
	if s.suspend == nil {
		return fn(ctx)
	}
	if !s.suspend(func() { err = fn(ctx) }) {
		return fn(ctx)
	}
	return err
}
