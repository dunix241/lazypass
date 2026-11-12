package bitwarden

import (
	"context"
	"encoding/json"
	"strings"

	"lazypass/internal/vault"
)

type containerTransfer struct {
	id, organizationID, source, target string
}

func (s snapshot) planFolderTransfer(source, destination vault.Path, name string, move bool) ([]containerTransfer, error) {
	if name == "" || (vault.Path{name}).Validate() != nil {
		return nil, vault.ErrInvalidPath
	}
	var containers []containerTransfer
	var selected, prefix, organizationID string
	if len(source) == 2 && source[0] == "Folders" && source[1] != noFolderID {
		for _, f := range s.folders {
			containers = append(containers, containerTransfer{id: f.ID, source: f.Name})
			if f.ID == source[1] {
				selected = f.Name
			}
		}
		if len(destination) == 2 && destination[0] == "Folders" && destination[1] != noFolderID {
			for _, f := range s.folders {
				if f.ID == destination[1] {
					prefix = f.Name
				}
			}
			if prefix == "" {
				return nil, vault.ErrNotFound
			}
		} else if len(destination) != 1 || destination[0] != "Folders" {
			return nil, vault.ErrUnsupported
		}
	} else if len(source) == 3 && source[0] == "Organizations" {
		c, err := s.organizationCollection(source)
		if err != nil {
			return nil, err
		}
		selected, organizationID = c.Name, c.OrganizationID
		if len(destination) == 3 && destination[0] == "Organizations" && destination[1] == organizationID {
			parent, err := s.organizationCollection(destination)
			if err != nil {
				return nil, err
			}
			prefix = parent.Name
		} else if len(destination) != 2 || destination[0] != "Organizations" || destination[1] != organizationID {
			return nil, vault.ErrUnsupported
		}
		for _, c := range s.collections {
			if c.OrganizationID == organizationID {
				containers = append(containers, containerTransfer{id: c.ID, organizationID: organizationID, source: c.Name})
			}
		}
	} else {
		return nil, vault.ErrUnsupported
	}
	if selected == "" {
		return nil, vault.ErrNotFound
	}
	if prefix == selected || strings.HasPrefix(prefix, selected+"/") {
		return nil, vault.ErrInvalidPath
	}
	target := name
	if prefix != "" {
		target = prefix + "/" + name
	}
	var plan []containerTransfer
	seen := map[string]bool{}
	for _, c := range containers {
		if c.source != selected && !strings.HasPrefix(c.source, selected+"/") {
			continue
		}
		if seen[c.source] {
			return nil, vault.ErrConflict
		}
		seen[c.source] = true
		c.target = target + strings.TrimPrefix(c.source, selected)
		for _, existing := range containers {
			if existing.source == c.target || strings.HasPrefix(existing.source, c.target+"/") {
				return nil, vault.ErrConflict
			}
		}
		plan = append(plan, c)
	}
	return plan, nil
}

func (p *Provider) checkFolderContents(ctx context.Context, s snapshot, plan []containerTransfer, move bool) error {
	if !move && len(plan) > 0 && plan[0].organizationID != "" {
		return p.checkCollectionCopy(ctx, s, plan)
	}
	if move {
		for _, c := range plan {
			if c.organizationID == "" {
				continue
			}
			for _, group := range s.collections {
				if group.ID == c.id && deniedCollection(group) {
					return permissionDenied("move collection")
				}
			}
			out, err := p.run(ctx, []string{"get", "org-collection", c.id, "--organizationid", c.organizationID}, nil)
			clear(out)
			if err != nil {
				return err
			}
		}
		return nil
	}
	ids := map[string]bool{}
	for _, c := range plan {
		ids[c.id] = true
	}
	for _, entry := range append(append([]item(nil), s.items...), s.excluded...) {
		if !ids[entry.FolderID] {
			continue
		}
		if entry.Type < 1 || entry.Type > 5 {
			return vault.ErrUnsupported
		}
		if _, err := p.transferItem(ctx, entry.ID, true); err != nil {
			return err
		}
	}
	return nil
}

func (p *Provider) CheckFolderTransfer(ctx context.Context, source, destination vault.Path, name string, move bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	job, err := p.loadHierarchy(ctx, p.hierarchyOperationPath(source, destination, name, move))
	if err != nil {
		return err
	}
	if job != nil {
		return nil
	}
	s, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	plan, err := s.planFolderTransfer(source, destination, name, move)
	if err != nil {
		return err
	}
	return p.checkFolderContents(ctx, s, plan, move)
}

func (p *Provider) TransferFolder(ctx context.Context, source, destination vault.Path, name string, move bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() { p.metadata = nil; p.browser.Store(nil) }()
	unlock, err := p.hierarchyLock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	journalPath := p.hierarchyOperationPath(source, destination, name, move)
	job, err := p.loadHierarchy(ctx, journalPath)
	if err != nil {
		return err
	}
	if job != nil {
		return p.runHierarchyCopy(ctx, journalPath, job)
	}
	s, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	plan, err := s.planFolderTransfer(source, destination, name, move)
	if err != nil {
		return err
	}
	if err := p.checkFolderContents(ctx, s, plan, move); err != nil {
		return err
	}
	if move {
		job, err = p.newMoveHierarchy(ctx, plan)
	} else {
		job, err = p.newHierarchy(ctx, s, plan)
	}
	if err != nil {
		return err
	}
	if err := saveHierarchy(journalPath, job); err != nil {
		return err
	}
	return p.runHierarchyCopy(ctx, journalPath, job)
}

func prepareClone(raw map[string]json.RawMessage) {
	for _, field := range []string{"id", "object", "revisionDate", "creationDate", "deletedDate", "attachments"} {
		delete(raw, field)
	}
}
