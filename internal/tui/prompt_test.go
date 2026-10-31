package tui

import (
	"context"
	"errors"
	"testing"

	"lazypass/internal/vault"
)

func TestPromptSuspendsOnlyWhenRequired(t *testing.T) {
	for _, mode := range []string{"cached", "error", "prompt", "refused", "none"} {
		t.Run(mode, func(t *testing.T) {
			calls, suspends := 0, 0
			s := &screen{suspend: func(fn func()) bool {
				suspends++
				if mode == "refused" {
					return false
				}
				fn()
				return true
			}}
			if mode == "none" {
				s.suspend = nil
			}
			failure := errors.New("operation failed")
			err := s.runPrompted(func(ctx context.Context) error {
				calls++
				if calls == 1 && !vault.IsNoninteractive(ctx) {
					t.Fatal("first attempt permits prompt")
				}
				if calls == 2 && vault.IsNoninteractive(ctx) {
					t.Fatal("retry still prohibits prompt")
				}
				if mode == "cached" {
					return nil
				}
				if mode == "error" {
					return failure
				}
				if vault.IsNoninteractive(ctx) {
					return vault.ErrPromptRequired
				}
				return nil
			})
			if mode == "cached" || mode == "error" {
				if calls != 1 || suspends != 0 {
					t.Fatal("unnecessary suspension/retry")
				}
			} else if calls != 2 {
				t.Fatal("missing interactive retry")
			}
			if mode == "error" {
				if !errors.Is(err, failure) {
					t.Fatal("error lost")
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
