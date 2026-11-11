package bitwarden

import (
	"context"
	"encoding/json"
	"lazypass/internal/vault"
	"slices"
)

type transferTarget struct {
	folderID   string
	collection collection
}

func (s snapshot) checkTransfer(source, destination vault.Path, name string, move bool) (item, transferTarget, error) {
	var target transferTarget
	if (vault.Path{name}).Validate() != nil || name == "" {
		return item{}, target, vault.ErrInvalidPath
	}
	entry, err := s.resolve(source)
	if err != nil {
		return entry, target, err
	}
	if denied(entry.Edit) && move {
		return entry, target, permissionDenied("move")
	}
	switch {
	case len(destination) == 2 && destination[0] == "Folders":
		target.folderID, err = s.folderID(destination[1])
		if err != nil {
			return entry, target, err
		}
		// Folder placement is account-local and does not change ownership.
		if !move && entry.OrganizationID != "" {
			return entry, target, vault.ErrUnsupported
		}
		if move && entry.FolderID == target.folderID {
			return entry, target, vault.ErrConflict
		}
	case len(destination) == 3 && destination[0] == "Organizations":
		target.collection, err = s.organizationCollection(destination)
		if err != nil {
			return entry, target, err
		}
		if deniedCollection(target.collection) {
			return entry, target, permissionDenied("paste into")
		}
		if entry.OrganizationID != target.collection.OrganizationID {
			return entry, target, vault.ErrUnsupported
		}
		if move {
			// Move only the membership represented by the source browser location.
			if len(source) != 4 || source[0] != "Organizations" || name != entry.Name {
				return entry, target, vault.ErrUnsupported
			}
			if source[2] == target.collection.ID || slices.Contains(entry.CollectionIDs, target.collection.ID) {
				return entry, target, vault.ErrConflict
			}
			if source[2] != "uncollected" {
				group, err := s.organizationCollection(source[:3])
				if err != nil {
					return entry, target, err
				}
				if deniedCollection(group) {
					return entry, target, permissionDenied("move")
				}
			}
		}
	default:
		return entry, target, vault.ErrUnsupported
	}
	nodes, err := s.list(destination)
	if err != nil {
		return entry, target, err
	}
	for _, node := range nodes {
		if node.Name == name {
			return entry, target, vault.ErrConflict
		}
	}
	return entry, target, nil
}

func deniedCollection(c collection) bool { return c.ReadOnly != nil && *c.ReadOnly }

func (p *Provider) CheckTransfer(ctx context.Context, source, destination vault.Path, name string, move bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	entry, _, err := s.checkTransfer(source, destination, name, move)
	if err != nil || move {
		return err
	}
	_, err = p.transferItem(ctx, entry.ID, true)
	return err
}

func (p *Provider) transferItem(ctx context.Context, id string, copying bool) (map[string]json.RawMessage, error) {
	output, err := p.run(ctx, []string{"get", "item", id}, nil)
	defer clear(output)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(output, &raw) != nil || raw == nil {
		return nil, vault.ErrMalformed
	}
	if copying {
		var hidden *bool
		_ = json.Unmarshal(raw["viewPassword"], &hidden)
		if denied(hidden) {
			return nil, permissionDenied("copy")
		}
		var attachments []json.RawMessage
		if value := raw["attachments"]; len(value) > 0 && json.Unmarshal(value, &attachments) != nil {
			return nil, vault.ErrMalformed
		}
		if len(attachments) > 0 {
			return nil, &commandFailure{vault.ErrUnsupported, "Copying entries with attachments is not supported"}
		}
	}
	return raw, nil
}

func (p *Provider) Transfer(ctx context.Context, source, destination vault.Path, name string, move bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() { p.metadata = nil; p.browser.Store(nil) }()
	s, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	entry, target, err := s.checkTransfer(source, destination, name, move)
	if err != nil {
		return err
	}
	if move && target.collection.ID != "" {
		ids := slices.Clone(entry.CollectionIDs)
		ids = slices.DeleteFunc(ids, func(id string) bool { return id == source[2] })
		ids = append(ids, target.collection.ID)
		output, err := p.submit(ctx, []string{"edit", "item-collections", entry.ID, "--organizationid", entry.OrganizationID}, ids)
		clear(output)
		return err
	}
	raw, err := p.transferItem(ctx, entry.ID, !move)
	if err != nil {
		return err
	}
	if !move {
		for _, field := range []string{"id", "object", "revisionDate", "creationDate", "deletedDate", "attachments"} {
			delete(raw, field)
		}
		raw["name"], _ = json.Marshal(name)
		if target.collection.ID != "" {
			raw["collectionIds"], _ = json.Marshal([]string{target.collection.ID})
			raw["folderId"] = json.RawMessage("null")
		}
	}
	if target.collection.ID == "" {
		raw["name"], _ = json.Marshal(name)
		if target.folderID == "" {
			raw["folderId"] = json.RawMessage("null")
		} else {
			raw["folderId"], _ = json.Marshal(target.folderID)
		}
	}
	args := []string{"create", "item"}
	if move {
		args = []string{"edit", "item", entry.ID}
	}
	result, err := p.submit(ctx, args, raw)
	clear(result)
	return err
}
