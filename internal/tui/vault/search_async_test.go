package vault

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

type slowSearchVault struct {
	memoryVault
	started  chan struct{}
	canceled chan struct{}
}

func TestTraversalPreservesPartialResultsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &cancelSearchVault{cancel: cancel}
	initial := []vault.Node{{Name: "folder", Path: vault.Path{"folder"}, Kind: vault.FolderNode}, {Name: "entry", Path: vault.Path{"entry"}, Kind: vault.EntryNode}}
	results, partial := collectSearchNodes(ctx, service.Service{Provider: p}, nil, initial)
	if !partial || len(results) != 2 {
		t.Fatalf("partial results lost: %v %t", results, partial)
	}
}

type cancelSearchVault struct {
	memoryVault
	cancel context.CancelFunc
}

func (p *cancelSearchVault) List(context.Context, vault.Path) ([]vault.Node, error) {
	p.cancel()
	return nil, errors.New("fixture canceled")
}

func (p *slowSearchVault) List(ctx context.Context, path vault.Path) ([]vault.Node, error) {
	if len(path) == 0 {
		return []vault.Node{{Name: "slow", Path: vault.Path{"slow"}, Kind: vault.FolderNode}}, nil
	}
	close(p.started)
	<-ctx.Done()
	close(p.canceled)
	return nil, ctx.Err()
}

func TestSearchDoesNotBlockInputAndCancels(t *testing.T) {
	p := &slowSearchVault{started: make(chan struct{}), canceled: make(chan struct{})}
	updates := make(chan func(), 1)
	v := New(Dependencies{Service: service.Service{Provider: p}, Dispatch: func(_ context.Context, fn func()) { updates <- fn }})
	v.Refresh()
	key(v, tcell.KeyRune, '/')
	select {
	case <-p.started:
	case <-time.After(time.Second):
		t.Fatal("search worker not started")
	}
	key(v, tcell.KeyRune, 'x')
	key(v, tcell.KeyEscape, 0)
	select {
	case <-p.canceled:
	case <-time.After(time.Second):
		t.Fatal("search not canceled")
	}
	if v.IsFiltering() || v.searching {
		t.Fatal("input blocked by search")
	}
}

func TestSearchDiscardsQueuedResultsAfterEscape(t *testing.T) {
	updates := make(chan func(), 1)
	p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "test", Path: vault.Path{"test"}, Kind: vault.EntryNode}}}}
	v := New(Dependencies{Service: service.Service{Provider: p}, Dispatch: func(_ context.Context, fn func()) { updates <- fn }})
	v.Refresh()
	key(v, tcell.KeyRune, '/')
	var update func()
	select {
	case update = <-updates:
	case <-time.After(time.Second):
		t.Fatal("missing search completion")
	}
	key(v, tcell.KeyEscape, 0)
	update()
	if v.searchLoaded || len(v.searchNodes) != 0 {
		t.Fatal("stale worker changed closed search")
	}
}
