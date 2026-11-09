package bitwarden

import (
	"context"
	"encoding/base64"
	"strings"

	"lazypass/internal/vault"
)

func (p *Provider) ContainerAction(path vault.Path) (string, bool) {
	return "Folder", true
}

type creationPlan struct{ organizationID, collectionName, entryName string }

func (s snapshot) planCreation(path vault.Path) (creationPlan, error) {
	var plan creationPlan
	if len(path) < 4 || path[0] != "Organizations" {
		return plan, vault.ErrInvalidPath
	}
	org := path[1]
	if _, ok := s.organizationTree[vault.Path{"Organizations", org}.String()]; !ok {
		org = ""
		for _, candidate := range s.organizations {
			if candidate.Name == path[1] {
				if org != "" {
					return plan, vault.ErrConflict
				}
				org = candidate.ID
			}
		}
		if org == "" {
			return plan, &commandFailure{vault.ErrUnsupported, "Organization does not exist; organization creation is not yet supported by the CLI integration"}
		}
	}
	plan.organizationID, plan.entryName = org, path[len(path)-1]
	parts := path[2 : len(path)-1]
	if group, ok := s.collectionByPath[vault.Path{"Organizations", org, parts[0]}.String()]; ok {
		if group.ReadOnly != nil && *group.ReadOnly {
			return plan, permissionDenied("create")
		}
		plan.collectionName = group.Name
		if len(parts) > 1 {
			plan.collectionName += "/" + parts[1:].String()
		}
	} else if strings.HasPrefix(parts[0], "group-") {
		location := vault.Path{"Organizations", org, parts[0]}
		if _, exists := s.organizationTree[location.String()]; !exists {
			return plan, vault.ErrNotFound
		}
		prefix, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(parts[0], "group-"))
		if err != nil {
			return plan, vault.ErrInvalidPath
		}
		plan.collectionName = string(prefix)
		if len(parts) > 1 {
			plan.collectionName += "/" + parts[1:].String()
		}
	} else {
		plan.collectionName = parts.String()
	}
	if _, direct := s.collectionByPath[vault.Path{"Organizations", org, parts[0]}.String()]; !direct || len(parts) > 1 {
		matches := 0
		for _, group := range s.collections {
			if group.OrganizationID == org && group.Name == plan.collectionName {
				matches++
				if group.ReadOnly != nil && *group.ReadOnly {
					return plan, permissionDenied("create")
				}
			}
		}
		if matches > 1 {
			return plan, vault.ErrConflict
		}
	}
	for _, entry := range s.excluded {
		if entry.OrganizationID != org || entry.Name != plan.entryName {
			continue
		}
		for _, id := range entry.CollectionIDs {
			for _, group := range s.collections {
				if group.ID == id && group.Name == plan.collectionName {
					return plan, vault.ErrConflict
				}
			}
		}
	}
	return plan, nil
}

func (p *Provider) ensureCollection(ctx context.Context, s *snapshot, org, name string) (collection, error) {
	var found *collection
	for i := range s.collections {
		c := &s.collections[i]
		if c.OrganizationID == org && c.Name == name {
			if found != nil {
				return collection{}, vault.ErrConflict
			}
			found = c
		}
	}
	if found != nil {
		return *found, nil
	}
	created, err := decode[collection](p.submit(ctx, []string{"create", "org-collection", "--organizationid", org}, map[string]any{"name": name, "organizationId": org, "groups": []any{}, "users": []any{}}))
	if err != nil {
		return collection{}, &commandFailure{err, "Collection creation failed; previously created parent collections may remain"}
	}
	if created.ID == "" {
		return collection{}, vault.ErrMalformed
	}
	created.OrganizationID, created.Name = org, name
	s.collections = append(s.collections, created)
	return created, nil
}
