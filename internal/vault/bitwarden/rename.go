package bitwarden

import (
	"context"
	"encoding/json"
	"lazypass/internal/vault"
)

func (p *Provider) Rename(ctx context.Context, node vault.Node, name string) error {
	if name == "" || (vault.Path{name}).Validate() != nil || len(node.Path) == 0 {
		return vault.ErrInvalidPath
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() { p.metadata = nil; p.browser.Store(nil) }()
	s, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	parent := node.Path[:len(node.Path)-1]
	nodes, err := s.list(parent)
	if err != nil {
		return err
	}
	for _, candidate := range nodes {
		if candidate.Name == name && candidate.Path.String() != node.Path.String() {
			return vault.ErrConflict
		}
	}
	if node.Kind == vault.EntryNode {
		entry, err := s.resolve(node.Path)
		if err != nil {
			return err
		}
		if denied(entry.Edit) {
			return permissionDenied("rename")
		}
		raw, err := p.transferItem(ctx, entry.ID, false)
		if err != nil {
			return err
		}
		raw["name"], _ = json.Marshal(name)
		out, err := p.submit(ctx, []string{"edit", "item", entry.ID}, raw)
		clear(out)
		return err
	}
	// Folder names may encode hierarchy; collection renames require separate
	// organization administration and are not implied by item edit permission.
	if len(node.Path) != 2 || node.Path[0] != "Folders" || node.Path[1] == noFolderID {
		return vault.ErrUnsupported
	}
	id, err := s.folderID(node.Path[1])
	if err != nil {
		return err
	}
	out, err := p.submit(ctx, []string{"edit", "folder", id}, map[string]string{"name": name})
	clear(out)
	return err
}
