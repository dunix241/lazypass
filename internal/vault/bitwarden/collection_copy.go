package bitwarden

import (
	"context"
	"lazypass/internal/vault"
)

// Empty assignments avoid copying sharing rules. The server still enforces
// organization roles and collection-creation policy.
func collectionClonePayload(c containerTransfer) map[string]any {
	return map[string]any{
		"name": c.target, "organizationId": c.organizationID,
		"users": []any{}, "groups": []any{},
	}
}

func (p *Provider) checkCollectionCopy(ctx context.Context, s snapshot, plan []containerTransfer) error {
	ids := make(map[string]bool, len(plan))
	for _, c := range plan {
		ids[c.id] = true
	}
	for _, entry := range s.excluded {
		for _, id := range entry.CollectionIDs {
			if !ids[id] {
				continue
			}
			if entry.Type < 1 || entry.Type > 5 {
				return vault.ErrUnsupported
			}
			if _, err := p.transferItem(ctx, entry.ID, true); err != nil {
				return err
			}
			break
		}
	}
	return nil
}
