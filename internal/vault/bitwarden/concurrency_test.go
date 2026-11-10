package bitwarden

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"lazypass/internal/vault"
)

type blockingDeleteRunner struct {
	fakeRunner
	started chan struct{}
	release chan struct{}
}

func TestCancelledBrowsingDoesNotReturnCachedRows(t *testing.T) {
	p := New(t.TempDir(), "", nil, &fakeRunner{})
	p.browser.Store(&snapshot{})
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.List(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled browse returned metadata: %v", err)
	}
}

func (r *blockingDeleteRunner) Run(ctx context.Context, args []string, in io.Reader, env []string) ([]byte, error) {
	if strings.Join(args, " ") == "delete item item-id" {
		close(r.started)
		select {
		case <-r.release:
			return []byte(`{}`), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return r.fakeRunner.Run(ctx, args, in, env)
}

func TestBrowsingSnapshotDoesNotWaitForDeletion(t *testing.T) {
	r := &blockingDeleteRunner{started: make(chan struct{}), release: make(chan struct{})}
	p := New(t.TempDir(), "", func() (string, error) { return "fixture", nil }, r)
	ctx := context.Background()
	if _, err := p.List(ctx, nil); err != nil {
		t.Fatal(err)
	}
	deleted := make(chan error, 1)
	go func() { deleted <- p.Delete(ctx, vault.Path{"hello", "world"}) }()
	select {
	case <-r.started:
	case <-time.After(time.Second):
		t.Fatal("deletion did not start")
	}
	defer func() {
		close(r.release)
		if err := <-deleted; err != nil {
			t.Error(err)
		}
	}()
	browsed := make(chan error, 1)
	go func() {
		nodes, err := p.List(ctx, vault.Path{"Folders", "folder-id"})
		if err == nil && len(nodes) != 1 {
			err = vault.ErrMalformed
		}
		browsed <- err
	}()
	select {
	case err := <-browsed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("navigation blocked behind deletion")
	}
	select {
	case <-deleted:
		t.Fatal("deletion unexpectedly completed")
	default:
	}
}
