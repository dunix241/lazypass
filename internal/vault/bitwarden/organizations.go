package bitwarden

import (
	"context"
	"sort"

	"lazypass/internal/vault"
)

type organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type collection struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	ReadOnly       *bool  `json:"readOnly"`
}

func personalPath(path vault.Path) vault.Path {
	if len(path) > 0 && path[0] == "Personal" {
		return path[1:]
	}
	return path
}

func permissionDenied(action string) error {
	return &commandFailure{vault.ErrUnsupported, "Your account cannot " + action + " this organization entry"}
}

func (s snapshot) organizationCollection(path vault.Path) (collection, error) {
	if len(path) != 3 {
		return collection{}, vault.ErrUnsupported
	}
	if c, ok := s.collectionByPath[path.String()]; ok {
		return c, nil
	}
	return collection{}, vault.ErrNotFound
}

func (s snapshot) resolveOrganizationItem(path vault.Path) (item, error) {
	if len(path) != 4 {
		return item{}, vault.ErrNotFound
	}
	if entry, ok := s.organizationItems[path.String()]; ok {
		return entry, nil
	}
	return item{}, vault.ErrNotFound
}

func (s snapshot) listOrganizations(path vault.Path) ([]vault.Node, error) {
	nodes, ok := s.organizationTree[path.String()]
	if !ok {
		return nil, vault.ErrNotFound
	}
	result := append([]vault.Node(nil), nodes...)
	for i := range result {
		result[i].Path = append(vault.Path(nil), result[i].Path...)
	}
	return result, nil
}

func (s *snapshot) indexOrganizations() {
	tree := make(map[string][]vault.Node)
	items := make(map[string]item)
	collections := make(map[string]collection, len(s.collections))
	root := vault.Path{"Organizations"}
	tree[root.String()] = nil
	addFolder := func(parent vault.Path, id, name string) vault.Path {
		path := append(append(vault.Path(nil), parent...), id)
		if path.Validate() != nil {
			return nil
		}
		if _, exists := tree[path.String()]; !exists {
			tree[parent.String()] = append(tree[parent.String()], vault.Node{Path: path, Name: name, Kind: vault.FolderNode, Protected: true})
			tree[path.String()] = nil
		}
		return path
	}
	for _, org := range s.organizations {
		addFolder(root, org.ID, org.Name)
	}
	for _, group := range s.collections {
		parent := vault.Path{"Organizations", group.OrganizationID}
		if _, ok := tree[parent.String()]; !ok {
			continue
		}
		path := addFolder(parent, group.ID, group.Name)
		if path != nil {
			collections[path.String()] = group
		}
	}
	for _, entry := range s.excluded {
		if entry.OrganizationID == "" || entry.Type < 1 || entry.Type > 5 || entry.ID == "" {
			continue
		}
		parent := vault.Path{"Organizations", entry.OrganizationID}
		if _, ok := tree[parent.String()]; !ok {
			continue
		}
		memberships := entry.CollectionIDs
		if len(memberships) == 0 {
			addFolder(parent, "uncollected", "Uncollected")
			memberships = []string{"uncollected"}
		}
		for _, id := range memberships {
			group := append(append(vault.Path(nil), parent...), id)
			if _, ok := tree[group.String()]; !ok {
				continue
			}
			path := append(group, entry.ID)
			if path.Validate() != nil {
				continue
			}
			key := path.String()
			if _, exists := items[key]; exists {
				continue
			}
			tree[group.String()] = append(tree[group.String()], vault.Node{Path: path, Name: entry.Name, Kind: vault.EntryNode, EntryType: entryType(entry.Type), TypeKnown: true, Shared: true})
			items[key] = entry
		}
	}
	for _, nodes := range tree {
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].Name == nodes[j].Name {
				return nodes[i].Path.String() < nodes[j].Path.String()
			}
			return nodes[i].Name < nodes[j].Name
		})
	}
	s.organizationTree, s.organizationItems = tree, items
	s.collectionByPath = collections
}

func (p *Provider) createOrganizationItem(ctx context.Context, s snapshot, entry vault.Entry) error {
	if len(entry.Path) != 4 {
		return vault.ErrUnsupported
	}
	c, err := s.organizationCollection(entry.Path[:3])
	if err != nil {
		return err
	}
	if c.ReadOnly != nil && *c.ReadOnly {
		return permissionDenied("create")
	}
	for _, existing := range s.excluded {
		if existing.OrganizationID == c.OrganizationID && existing.Name == entry.Path[3] {
			for _, id := range existing.CollectionIDs {
				if id == c.ID {
					return vault.ErrConflict
				}
			}
		}
	}
	payload := itemPayload(entry, entry.Path[3], "")
	payload["organizationId"], payload["collectionIds"] = c.OrganizationID, []string{c.ID}
	_, err = p.submit(ctx, []string{"create", "item"}, payload)
	return err
}

func itemPayload(entry vault.Entry, name, folderID string) map[string]any {
	payload := map[string]any{"type": bitwardenType(entry.Type), "name": name, "folderId": folderID, "notes": entry.Notes}
	switch entry.Type {
	case vault.LoginEntry:
		payload["login"] = map[string]any{"username": entry.Username, "password": entry.Password, "uris": []map[string]string{{"uri": entry.URL}}}
	case vault.NoteEntry:
		payload["secureNote"] = map[string]int{"type": 0}
	default:
		payload[typeObject(entry.Type)] = entry.Fields
	}
	return payload
}
