package bitwarden

import (
	"context"
	"encoding/base64"
	"sort"
	"strings"

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
	byName := make(map[string][]collection)
	for _, group := range s.collections {
		key := group.OrganizationID + "\x00" + group.Name
		byName[key] = append(byName[key], group)
	}
	root := vault.Path{"Organizations"}
	tree[root.String()] = nil
	addFolder := func(parent vault.Path, id, name string) vault.Path {
		path := append(append(vault.Path(nil), parent...), id)
		if path.Validate() != nil {
			return nil
		}
		if _, exists := tree[path.String()]; !exists {
			tree[parent.String()] = append(tree[parent.String()], vault.Node{Path: path, Name: name, Kind: vault.FolderNode, Protected: len(path) != 3 || id == "uncollected"})
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
	// Attach to the nearest actual ancestor; missing ancestors stay in the label.
	for _, group := range s.collections {
		orgPath := vault.Path{"Organizations", group.OrganizationID}
		original := vault.Path{"Organizations", group.OrganizationID, group.ID}
		if _, ok := collections[original.String()]; !ok {
			continue
		}
		parts := strings.Split(group.Name, "/")
		parent := orgPath
		labelStart := 0
		for i := len(parts) - 2; i >= 0; i-- {
			prefix := strings.Join(parts[:i+1], "/")
			if matches := byName[group.OrganizationID+"\x00"+prefix]; len(matches) == 1 {
				parent = vault.Path{"Organizations", group.OrganizationID, matches[0].ID}
				labelStart = i + 1
				break
			}
		}
		if len(parts) > 1 {
			nodes := tree[orgPath.String()]
			for i, node := range nodes {
				if node.Path.String() == original.String() {
					nodes = append(nodes[:i], nodes[i+1:]...)
					break
				}
			}
			tree[orgPath.String()] = nodes
			tree[parent.String()] = append(tree[parent.String()], vault.Node{Path: original, Name: strings.Join(parts[labelStart:], "/"), Kind: vault.FolderNode})
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
	plan, err := s.planCreation(entry.Path)
	if err != nil {
		return err
	}
	var c collection
	if len(entry.Path) == 4 {
		c, _ = s.organizationCollection(entry.Path[:3])
	}
	if c.ID == "" {
		parts := strings.Split(plan.collectionName, "/")
		for i := range parts {
			c, err = p.ensureCollection(ctx, &s, plan.organizationID, strings.Join(parts[:i+1], "/"))
			if err != nil {
				return err
			}
		}
	}
	payload := itemPayload(entry, plan.entryName, "")
	payload["organizationId"], payload["collectionIds"] = c.OrganizationID, []string{c.ID}
	_, err = p.submit(ctx, []string{"create", "item"}, payload)
	if err == nil {
		if _, syncErr := p.run(ctx, []string{"sync"}, nil); syncErr != nil {
			return vault.SavedWarning{Message: "Entry saved, but refresh failed; refresh before retrying creation"}
		}
	}
	return err
}

func (p *Provider) ValidateDestination(ctx context.Context, path vault.Path) error {
	if len(path) == 0 {
		return vault.ErrInvalidPath
	}
	if path[0] != "Organizations" {
		if path[0] == "Folders" {
			if len(path) < 3 {
				return vault.ErrInvalidPath
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			s, err := p.snapshot(ctx)
			if err != nil {
				return err
			}
			parent, err := s.folderNamePath(path)
			if err != nil {
				return err
			}
			return s.checkFolderDestination(parent)
		}
		if len(personalPath(path)) == 0 {
			return vault.ErrInvalidPath
		}
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	_, err = s.planCreation(path)
	return err
}

func (p *Provider) createCollection(ctx context.Context, path vault.Path) error {
	if len(path) < 3 {
		return vault.ErrInvalidPath
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() { p.metadata = nil }()
	s, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	name := path[2:].String()
	if _, ok := s.organizationTree[path[:2].String()]; !ok {
		id := ""
		for _, org := range s.organizations {
			if org.Name == path[1] {
				if id != "" {
					return vault.ErrConflict
				}
				id = org.ID
			}
		}
		if id == "" {
			return &commandFailure{vault.ErrUnsupported, "Organization does not exist; create it in the web vault first"}
		}
		path = append(vault.Path(nil), path...)
		path[1] = id
	}
	if len(path) > 3 {
		if parent, err := s.organizationCollection(path[:3]); err == nil {
			name = parent.Name + "/" + path[3:].String()
		} else if strings.HasPrefix(path[2], "group-") {
			prefix, decodeErr := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(path[2], "group-"))
			if decodeErr != nil {
				return vault.ErrInvalidPath
			}
			if _, ok := s.organizationTree[path[:3].String()]; !ok {
				return vault.ErrNotFound
			}
			name = string(prefix) + "/" + path[3:].String()
		}
	}
	if _, ok := s.organizationTree[path[:2].String()]; !ok {
		return vault.ErrNotFound
	}
	for _, c := range s.collections {
		if c.OrganizationID == path[1] && c.Name == name {
			return vault.ErrConflict
		}
	}
	_, err = p.submit(ctx, []string{"create", "org-collection", "--organizationid", path[1]}, map[string]any{"name": name, "organizationId": path[1], "groups": []any{}, "users": []any{}})
	if err == nil {
		return p.refreshCreatedContainer(ctx)
	}
	return err
}

func itemPayload(entry vault.Entry, name, folderID string) map[string]any {
	payload := map[string]any{"type": bitwardenType(entry.Type), "name": name, "folderId": folderID, "notes": entry.Notes}
	switch entry.Type {
	case vault.LoginEntry:
		payload["login"] = map[string]any{
			"username": entry.Username,
			"password": entry.Password,
			"totp":     entry.Authenticator,
			"uris":     []map[string]string{{"uri": entry.URL}},
		}
	case vault.NoteEntry:
		payload["secureNote"] = map[string]int{"type": 0}
	default:
		payload[typeObject(entry.Type)] = entry.Fields
	}
	return payload
}
