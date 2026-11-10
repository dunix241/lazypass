package bitwarden

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"lazypass/internal/vault"
)

type parallelCodeRunner struct {
	fakeRunner
	started chan struct{}
	release chan struct{}
}

func (r *parallelCodeRunner) Run(ctx context.Context, args []string, input io.Reader, env []string) ([]byte, error) {
	if strings.Join(args, " ") == "get totp item-id" {
		r.started <- struct{}{}
		select {
		case <-r.release:
			return []byte("123456"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return r.fakeRunner.Run(ctx, args, input, env)
}

func TestAuthenticatorCommandsOverlapAndLockWaitsForReaders(t *testing.T) {
	r := &parallelCodeRunner{started: make(chan struct{}, 2), release: make(chan struct{})}
	p := New(t.TempDir(), "", func() (string, error) { return "fixture", nil }, r)
	if _, err := p.List(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := p.AuthenticatorCode(context.Background(), vault.Path{"hello", "world"})
			results <- err
		}()
	}
	for range 2 {
		select {
		case <-r.started:
		case <-time.After(time.Second):
			t.Fatal("native commands serialized")
		}
	}
	locked := make(chan struct{})
	go func() { p.Lock(); close(locked) }()
	select {
	case <-locked:
		t.Fatal("lock bypassed active readers")
	default:
	}
	close(r.release)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-locked:
	case <-time.After(time.Second):
		t.Fatal("lock did not complete")
	}
}
