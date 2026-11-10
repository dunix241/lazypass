package service

import (
	"context"
	"errors"
	"testing"

	"lazypass/internal/vault"
)

type codeProvider struct {
	copyProvider
	cancel context.CancelFunc
	err    error
}

func (p codeProvider) AuthenticatorCode(context.Context, vault.Path) (string, error) {
	if p.cancel != nil {
		p.cancel()
	}
	return "123456", p.err
}

func TestAuthenticatorDelegationCancellationAndNoFallback(t *testing.T) {
	for _, fail := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		p := codeProvider{}
		if fail {
			p.err = vault.ErrUnsupported
		} else {
			p.cancel = cancel
		}
		copied := false
		s := Service{Provider: p, Copy: func(string) error { copied = true; return nil }}
		err := s.CopyAuthenticator(ctx, vault.Path{"test"})
		cancel()
		if copied || err == nil || (fail && !errors.Is(err, vault.ErrUnsupported)) {
			t.Fatal("canceled or refused code reached clipboard")
		}
	}
}

func TestClipboardChecksOriginalCancellationBeforeAsyncPropagation(t *testing.T) {
	guard, cancel := context.WithCancel(context.Background())
	cancel()
	for _, authenticator := range []bool{false, true} {
		copied := false
		s := Service{
			Provider: codeProvider{},
			Copy:     func(string) error { copied = true; return nil },
		}
		var err error
		if authenticator {
			err = s.CopyAuthenticator(context.Background(), vault.Path{"test"}, guard)
		} else {
			err = s.CopyPassword(context.Background(), vault.Path{"test"}, guard)
		}
		if err != context.Canceled || copied {
			t.Fatal("original cancellation did not prevent clipboard mutation")
		}
	}
}
