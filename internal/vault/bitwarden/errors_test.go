package bitwarden

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"lazypass/internal/vault"
)

func TestTimeoutHasSafeActionableMessage(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	err := classifyFailure(ctx, []byte("private-fixture"), errors.New("external timeout"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("timeout cause lost")
	}
	diagnostic, ok := err.(vault.Diagnostic)
	if !ok || !strings.Contains(diagnostic.UserMessage(), "timed out") || strings.Contains(err.Error(), "private-fixture") {
		t.Fatal("unsafe or missing timeout message")
	}
}

func TestCommandFailuresAreClassifiedWithoutPayload(t *testing.T) {
	for _, test := range []struct {
		output string
		cause  error
	}{{"You are not logged in", vault.ErrUninitialized}, {"Invalid master password", vault.ErrLocked}, {"Invalid session", vault.ErrLocked}, {"fetch failed ECONNREFUSED", vault.ErrUnavailable}, {"SSH key fingerprint is required", vault.ErrMalformed}, {"unexpected failure", vault.ErrUnavailable}} {
		err := classifyFailure(context.Background(), []byte(test.output+" private-fixture-must-not-leak"), errors.New("external command failed"))
		if !errors.Is(err, test.cause) || strings.Contains(err.Error(), "private-fixture") {
			t.Fatalf("unsafe or incorrect classification: %v", err)
		}
		if _, ok := err.(vault.Diagnostic); !ok {
			t.Fatal("failure has no safe user message")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := classifyFailure(ctx, nil, errors.New("failed")); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost")
	}
}
