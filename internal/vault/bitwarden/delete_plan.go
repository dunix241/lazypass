package bitwarden

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"lazypass/internal/vault"
)

type collectionDeletion struct {
	plan               vault.DeletePlan
	items, collections []string
}

func (s snapshot) collectionDeletion(path vault.Path) (collectionDeletion, error) {
	var result collectionDeletion
	selected, err := s.organizationCollection(path)
	if err != nil {
		return result, err
	}
	ids := make(map[string]bool)
	for _, group := range s.collections {
		if group.OrganizationID == selected.OrganizationID && (group.ID == selected.ID || strings.HasPrefix(group.Name, selected.Name+"/")) {
			if group.ReadOnly != nil && *group.ReadOnly {
				return result, permissionDenied("delete")
			}
			ids[group.ID] = true
			result.collections = append(result.collections, group.ID)
		}
	}
	seen := make(map[string]bool)
	for _, item := range s.excluded {
		if item.OrganizationID != selected.OrganizationID {
			continue
		}
		for _, id := range item.CollectionIDs {
			if ids[id] && !seen[item.ID] {
				if item.Edit != nil && !*item.Edit {
					return result, permissionDenied("delete")
				}
				seen[item.ID] = true
				result.items = append(result.items, item.ID)
			}
		}
	}
	sort.Strings(result.items)
	sort.Strings(result.collections)
	encoded, _ := json.Marshal(struct {
		Target             vault.Path
		Name               string
		Items, Collections []string
	}{path, selected.Name, result.items, result.collections})
	digest := sha256.Sum256(encoded)
	result.plan = vault.DeletePlan{Target: append(vault.Path(nil), path...), Entries: len(result.items), Containers: len(result.collections), Shared: true, Token: hex.EncodeToString(digest[:])}
	return result, nil
}

func (p *Provider) PlanDelete(ctx context.Context, path vault.Path) (vault.DeletePlan, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(path) != 3 || path[0] != "Organizations" {
		return vault.DeletePlan{}, vault.ErrUnsupported
	}
	s, err := p.deletionSnapshot(ctx)
	if err != nil {
		return vault.DeletePlan{}, err
	}
	result, err := s.collectionDeletion(path)
	return result.plan, err
}

func (p *Provider) ExecuteDelete(ctx context.Context, plan vault.DeletePlan) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() { p.metadata = nil }()
	if len(plan.Target) != 3 || plan.Target[0] != "Organizations" || plan.Token == "" {
		return vault.ErrInvalidPath
	}
	s, err := p.deletionSnapshot(ctx)
	if err != nil {
		return err
	}
	current, err := s.collectionDeletion(plan.Target)
	if err != nil {
		return err
	}
	if current.plan.Token != plan.Token {
		return &commandFailure{vault.ErrConflict, "Deletion scope changed; review and confirm again"}
	}
	depth := make(map[string]int, len(s.collections))
	for _, collection := range s.collections {
		depth[collection.ID] = strings.Count(collection.Name, "/")
	}
	sort.SliceStable(current.collections, func(i, j int) bool { return depth[current.collections[i]] > depth[current.collections[j]] })
	for _, id := range current.items {
		if _, err := p.run(ctx, []string{"delete", "item", id}, nil); err != nil {
			return &commandFailure{err, "Deletion stopped; some entries may already have been deleted. Refresh before retrying"}
		}
	}
	for _, id := range current.collections {
		if _, err := p.run(ctx, []string{"delete", "org-collection", id, "--organizationid", plan.Target[1]}, nil); err != nil {
			return &commandFailure{err, "Deletion stopped; some collections may remain. Refresh before retrying"}
		}
	}
	_, err = p.run(ctx, []string{"sync"}, nil)
	if err != nil {
		return &commandFailure{err, "Deletion completed, but refresh failed; refresh the vault before retrying"}
	}
	return err
}

// Caller holds p.mu; both confirmation and execution require a fresh snapshot.
func (p *Provider) deletionSnapshot(ctx context.Context) (snapshot, error) {
	p.metadata = nil
	if err := ctx.Err(); err != nil {
		return snapshot{}, err
	}
	if err := p.ensureUnlocked(ctx); err != nil {
		return snapshot{}, err
	}
	if _, err := p.run(ctx, []string{"sync"}, nil); err != nil {
		return snapshot{}, err
	}
	return p.snapshot(ctx)
}
