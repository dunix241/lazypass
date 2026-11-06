package vault

import (
	"context"
	"testing"

	"lazypass/internal/tui/shared"
)

type blockedAccessError struct{}

func (blockedAccessError) Error() string { return "blocked fixture" }
func (blockedAccessError) UserMessage() string {
	return "Server access denied; disconnect VPN and refresh"
}

func TestRefreshPreservesAccessWarning(t *testing.T) {
	var level shared.Level
	var message string
	v := New(Dependencies{
		RunOperation: func(_ func(context.Context) error, done func(error)) { done(blockedAccessError{}) },
		Notify:       func(l shared.Level, text string) { level, message = l, text },
	})
	v.refreshProvider()
	if level != shared.Warning || message != (blockedAccessError{}).UserMessage() {
		t.Fatalf("access warning replaced: level=%v message=%q", level, message)
	}
	if v.refreshing {
		t.Fatal("failed refresh left busy state active")
	}
}
