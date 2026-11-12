package bitwarden

import (
	"context"
	"encoding/json"
	"lazypass/internal/vault"
)

func (p *Provider) runHierarchyMove(ctx context.Context, path string, job *hierarchyJob) error {
	if _, err := p.run(ctx, []string{"sync"}, nil); err != nil {
		return err
	}
	for i := range job.Containers {
		c := &job.Containers[i]
		if c.TargetID != "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		get, args := []string{"get", "folder", c.SourceID}, []string{"edit", "folder", c.SourceID}
		if c.OrganizationID != "" {
			get = []string{"get", "org-collection", c.SourceID, "--organizationid", c.OrganizationID}
			args = []string{"edit", "org-collection", c.SourceID, "--organizationid", c.OrganizationID}
		}
		raw, err := decode[map[string]json.RawMessage](p.run(ctx, get, nil))
		if err != nil {
			return err
		}
		if raw == nil {
			return vault.ErrMalformed
		}
		var name string
		if json.Unmarshal(raw["name"], &name) != nil {
			return vault.ErrMalformed
		}
		if name == c.Name {
			if !c.Attempted {
				return vault.ErrConflict
			}
			c.TargetID = c.SourceID
			if err := saveHierarchy(path, job); err != nil {
				return err
			}
			continue
		}
		if name != c.OriginalName {
			return &commandFailure{vault.ErrConflict, "Source container changed outside this transfer; move stopped without overwriting"}
		}
		if c.Attempted {
			return &commandFailure{vault.ErrUnavailable, "Move response remains unresolved; no write was repeated"}
		}
		c.Attempted = true
		if err := saveHierarchy(path, job); err != nil {
			return err
		}
		raw["name"], _ = json.Marshal(c.Name)
		out, err := p.submit(ctx, args, raw)
		clear(out)
		if err != nil {
			return err
		}
		c.TargetID = c.SourceID
		if err := saveHierarchy(path, job); err != nil {
			return err
		}
	}
	job.Complete = true
	return saveHierarchy(path, job)
}
