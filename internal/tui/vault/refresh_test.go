package vault

import (
	"context"
	"testing"

	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

type syncingVault struct {
	memoryVault
	syncs int
}

func (p *syncingVault) Capabilities() vault.Capabilities { return vault.Capabilities{Sync: true} }
func (p *syncingVault) Sync(context.Context) (vault.SyncResult, error) {
	p.syncs++
	return vault.SyncResult{Pulled: true}, nil
}

func TestRefreshSyncsOnceAndThenReloads(t *testing.T) {
	var work func(context.Context) error
	var done func(error)
	p := &syncingVault{}
	v := New(Dependencies{Service: service.Service{Provider: p}, RunOperation: func(w func(context.Context) error, d func(error)) { work, done = w, d }})
	v.refreshProvider()
	v.refreshProvider()
	if !v.refreshing || p.syncs != 0 {
		t.Fatal("refresh did not yield")
	}
	err := work(context.Background())
	v.deps.RunOperation = nil
	done(err)
	if p.syncs != 1 || v.refreshing {
		t.Fatal("refresh duplicated or did not complete")
	}
}
