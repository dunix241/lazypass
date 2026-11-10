package service

import (
	"context"
	"testing"

	"lazypass/internal/vault"
)

type importProvider struct {
	copyProvider
	created int
	cancel  context.CancelFunc
}

func (p *importProvider) Create(context.Context, vault.Entry) error {
	p.created++
	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

func TestImportCancellationPreservesConfirmedResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &importProvider{cancel: cancel}
	s := Service{Provider: p}
	results := s.ImportEntries(ctx, []vault.Entry{
		{Path: vault.Path{"first"}},
		{Path: vault.Path{"second"}},
	}, nil)
	if p.created != 1 || !results[0].Created || results[1].Created || results[1].Status != "Canceled; not attempted" {
		t.Fatal("cancellation lost successful result or continued creating")
	}
}
