package bitwarden

import (
	"context"
	"lazypass/internal/vault"
)

func (p *Provider) ChangesOwnership(ctx context.Context, source, destination vault.Path, move bool) (bool, error) {
	if !move || len(destination) != 3 || destination[0] != "Organizations" {
		return false, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.snapshot(ctx)
	if err != nil {
		return false, err
	}
	entry, err := s.resolve(source)
	if err != nil {
		return false, err
	}
	return entry.OrganizationID == "", nil
}
