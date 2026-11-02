package bitwarden

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"

	"lazypass/internal/vault"
)

type folder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type item struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	FolderID       string  `json:"folderId"`
	OrganizationID string  `json:"organizationId"`
	Type           int     `json:"type"`
	DeletedDate    *string `json:"deletedDate"`
	Login          struct {
		Username string `json:"username"`
		Password string `json:"password"`
		URIs     []struct {
			URI string `json:"uri"`
		} `json:"uris"`
	} `json:"login"`
	Notes string `json:"notes"`
}

type snapshot struct {
	folders  []folder
	items    []item
	excluded []item
}

func (p *Provider) SupportedEntryTypes() []vault.EntryType {
	return vault.AllEntryTypes()
}

func (p *Provider) snapshot(ctx context.Context) (snapshot, error) {
	if ctx.Err() != nil {
		return snapshot{}, vault.ErrUnavailable
	}
	if p.metadata != nil {
		return *p.metadata, nil
	}
	var result snapshot
	if err := p.ensureUnlocked(ctx); err != nil {
		return result, err
	}
	folders, err := decode[[]folder](p.run(ctx, []string{"list", "folders"}, nil))
	if err != nil {
		return result, err
	}
	items, err := decode[[]item](p.run(ctx, []string{"list", "items"}, nil))
	if err != nil {
		return result, err
	}
	result.folders = folders
	for _, entry := range items {
		entry.Login.Password = ""
		entry.Login.Username = ""
		entry.Login.URIs = nil
		entry.Notes = ""
		if entry.Type >= 1 && entry.Type <= 5 && entry.OrganizationID == "" && entry.DeletedDate == nil {
			result.items = append(result.items, entry)
		} else if entry.DeletedDate == nil {
			result.excluded = append(result.excluded, entry)
		}
	}
	p.metadata = &result
	return result, nil
}

func (s snapshot) path(entry item) (vault.Path, bool) {
	var path vault.Path
	if entry.FolderID != "" {
		found := false
		for _, f := range s.folders {
			if f.ID == entry.FolderID {
				var err error
				path, err = vault.ParsePath(f.Name)
				if err != nil {
					return nil, false
				}
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	path = append(path, entry.Name)
	return path, path.Validate() == nil && entry.Name != ""
}

func (s snapshot) resolve(path vault.Path) (item, error) {
	var matches []item
	for _, entry := range s.items {
		candidate, ok := s.path(entry)
		if ok && candidate.String() == path.String() {
			matches = append(matches, entry)
		}
	}
	if len(matches) == 0 {
		return item{}, vault.ErrNotFound
	}
	if len(matches) > 1 {
		return item{}, vault.ErrConflict
	}
	return matches[0], nil
}

func validPath(path vault.Path) error {
	if len(path) == 0 {
		return vault.ErrInvalidPath
	}
	return path.Validate()
}

func (p *Provider) List(ctx context.Context, path vault.Path) ([]vault.Node, error) {
	if err := path.Validate(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	folders := map[string]vault.Node{}
	var nodes []vault.Node
	prefix := path.String()
	if prefix != "" {
		prefix += "/"
	}
	addFolder := func(value string) {
		if !strings.HasPrefix(value, prefix) {
			return
		}
		tail := strings.TrimPrefix(value, prefix)
		name, _, _ := strings.Cut(tail, "/")
		if name == "" {
			return
		}
		child := append(append(vault.Path(nil), path...), name)
		if child.Validate() != nil {
			return
		}
		folders[name] = vault.Node{Path: child, Name: name, Kind: vault.FolderNode}
	}
	for _, f := range s.folders {
		if parsed, err := vault.ParsePath(f.Name); err == nil && len(parsed) > 0 {
			addFolder(f.Name)
		}
	}
	for _, entry := range s.items {
		candidate, ok := s.path(entry)
		if !ok {
			continue
		}
		if len(candidate) > len(path)+1 {
			addFolder(candidate[:len(candidate)-1].String())
			continue
		}
		if len(candidate) == len(path)+1 && strings.HasPrefix(candidate.String(), prefix) {
			nodes = append(nodes, vault.Node{Path: candidate, Name: entry.Name, Kind: vault.EntryNode, EntryType: entryType(entry.Type), TypeKnown: true})
		}
	}
	// Duplicate names remain visible; operations reject ambiguity rather than
	// updating whichever server item happens to be returned first.
	for _, f := range folders {
		nodes = append(nodes, f)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Kind != nodes[j].Kind {
			return nodes[i].Kind == vault.FolderNode
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

func (p *Provider) Read(ctx context.Context, path vault.Path) (vault.Entry, error) {
	if err := validPath(path); err != nil {
		return vault.Entry{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.snapshot(ctx)
	if err != nil {
		return vault.Entry{}, err
	}
	entry, err := s.resolve(path)
	if err != nil {
		return vault.Entry{}, err
	}
	entry, err = decode[item](p.run(ctx, []string{"get", "item", entry.ID}, nil))
	if err != nil {
		return vault.Entry{}, err
	}
	result := vault.Entry{Path: append(vault.Path(nil), path...), Password: entry.Login.Password, Username: entry.Login.Username, Notes: entry.Notes}
	result.Type = entryType(entry.Type)
	if result.Type != vault.LoginEntry {
		raw, err := decode[map[string]json.RawMessage](p.run(ctx, []string{"get", "item", entry.ID}, nil))
		if err != nil {
			return vault.Entry{}, err
		}
		var values map[string]json.RawMessage
		if json.Unmarshal(raw[typeObject(result.Type)], &values) != nil {
			return vault.Entry{}, vault.ErrMalformed
		}
		result.Fields = map[string]string{}
		for key, value := range values {
			var text string
			if json.Unmarshal(value, &text) == nil {
				result.Fields[key] = text
			}
		}
	}
	if len(entry.Login.URIs) > 0 {
		result.URL = entry.Login.URIs[0].URI
	}
	return result, nil
}

func (p *Provider) submit(ctx context.Context, args []string, value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, vault.ErrMalformed
	}
	defer clear(data)
	encoded := base64.StdEncoding.EncodeToString(data)
	return p.run(ctx, args, strings.NewReader(encoded))
}

func (p *Provider) ensureFolder(ctx context.Context, s *snapshot, path vault.Path) (string, error) {
	if len(path) == 0 {
		return "", nil
	}
	name := path.String()
	id := ""
	for _, f := range s.folders {
		if f.Name == name {
			if id != "" {
				return "", vault.ErrConflict
			}
			id = f.ID
		}
	}
	if id != "" {
		return id, nil
	}
	created, err := decode[folder](p.submit(ctx, []string{"create", "folder"}, map[string]string{"name": name}))
	if err != nil {
		return "", err
	}
	if created.ID == "" {
		return "", vault.ErrMalformed
	}
	s.folders = append(s.folders, created)
	return created.ID, nil
}

func (p *Provider) Create(ctx context.Context, entry vault.Entry) error {
	if err := validPath(entry.Path); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() { p.metadata = nil }()
	s, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	if _, err := s.resolve(entry.Path); err != vault.ErrNotFound {
		return vault.ErrConflict
	}
	id, err := p.ensureFolder(ctx, &s, entry.Path[:len(entry.Path)-1])
	if err != nil {
		return err
	}
	payload := map[string]any{"type": bitwardenType(entry.Type), "name": entry.Path[len(entry.Path)-1], "folderId": id, "notes": entry.Notes}
	if entry.Type == vault.LoginEntry {
		payload["login"] = map[string]any{"username": entry.Username, "password": entry.Password, "uris": []map[string]string{{"uri": entry.URL}}}
	} else if entry.Type == vault.NoteEntry {
		payload["secureNote"] = map[string]int{"type": 0}
	} else {
		payload[typeObject(entry.Type)] = entry.Fields
	}
	_, err = p.submit(ctx, []string{"create", "item"}, payload)
	return err
}

func (p *Provider) Write(ctx context.Context, entry vault.Entry) error {
	if err := validPath(entry.Path); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() { p.metadata = nil }()
	s, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	existing, err := s.resolve(entry.Path)
	if err != nil {
		return err
	}
	// Fetch and patch the complete object so unknown fields, attachments, custom
	// fields, TOTP and additional URIs survive value-only edits.
	raw, err := decode[map[string]json.RawMessage](p.run(ctx, []string{"get", "item", existing.ID}, nil))
	if err != nil {
		return err
	}
	if entry.Type != entryType(existing.Type) {
		return vault.ErrUnsupported
	}
	if entry.Type != vault.LoginEntry {
		if entry.Type != vault.NoteEntry {
			object := typeObject(entry.Type)
			values := map[string]json.RawMessage{}
			if value := raw[object]; len(value) > 0 && string(value) != "null" {
				if json.Unmarshal(value, &values) != nil {
					return vault.ErrMalformed
				}
			}
			for key, value := range entry.Fields {
				values[key], _ = json.Marshal(value)
			}
			raw[object], _ = json.Marshal(values)
		}
		raw["notes"], _ = json.Marshal(entry.Notes)
		_, err = p.submit(ctx, []string{"edit", "item", existing.ID}, raw)
		return err
	}
	var login map[string]json.RawMessage
	if json.Unmarshal(raw["login"], &login) != nil || login == nil {
		return vault.ErrMalformed
	}
	login["password"], _ = json.Marshal(entry.Password)
	login["username"], _ = json.Marshal(entry.Username)
	var uris []map[string]json.RawMessage
	if value, ok := login["uris"]; ok {
		if err := json.Unmarshal(value, &uris); err != nil {
			return vault.ErrMalformed
		}
	}
	currentURL := ""
	if len(uris) > 0 {
		_ = json.Unmarshal(uris[0]["uri"], &currentURL)
	}
	if entry.URL != currentURL {
		if len(uris) == 0 {
			uris = append(uris, map[string]json.RawMessage{})
		}
		if uris[0] == nil {
			uris[0] = map[string]json.RawMessage{}
		}
		uris[0]["uri"], _ = json.Marshal(entry.URL)
		login["uris"], _ = json.Marshal(uris)
	}
	raw["notes"], _ = json.Marshal(entry.Notes)
	raw["login"], _ = json.Marshal(login)
	_, err = p.submit(ctx, []string{"edit", "item", existing.ID}, raw)
	return err
}

func entryType(value int) vault.EntryType {
	switch value {
	case 2:
		return vault.NoteEntry
	case 3:
		return vault.CardEntry
	case 4:
		return vault.IdentityEntry
	case 5:
		return vault.SSHKeyEntry
	default:
		return vault.LoginEntry
	}
}
func bitwardenType(value vault.EntryType) int {
	switch value {
	case vault.NoteEntry:
		return 2
	case vault.CardEntry:
		return 3
	case vault.IdentityEntry:
		return 4
	case vault.SSHKeyEntry:
		return 5
	default:
		return 1
	}
}
func typeObject(value vault.EntryType) string {
	switch value {
	case vault.CardEntry:
		return "card"
	case vault.IdentityEntry:
		return "identity"
	case vault.SSHKeyEntry:
		return "sshKey"
	case vault.NoteEntry:
		return "secureNote"
	default:
		return "login"
	}
}

func (p *Provider) Delete(ctx context.Context, path vault.Path) error {
	if err := validPath(path); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() { p.metadata = nil }()
	s, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	entry, err := s.resolve(path)
	if err != nil {
		return err
	}
	_, err = p.run(ctx, []string{"delete", "item", entry.ID}, nil)
	return err
}

func (p *Provider) CreateFolder(ctx context.Context, path vault.Path) error {
	if err := validPath(path); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() { p.metadata = nil }()
	s, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	_, err = p.ensureFolder(ctx, &s, path)
	return err
}

func (p *Provider) DeleteFolder(ctx context.Context, path vault.Path) error {
	if err := validPath(path); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() { p.metadata = nil }()
	s, err := p.snapshot(ctx)
	if err != nil {
		return err
	}
	prefix := path.String() + "/"
	for _, entry := range s.excluded {
		for _, f := range s.folders {
			if f.ID == entry.FolderID && (f.Name == path.String() || strings.HasPrefix(f.Name, prefix)) {
				return vault.ErrUnsupported
			}
		}
	}
	for _, entry := range s.items {
		candidate, ok := s.path(entry)
		if ok && strings.HasPrefix(candidate.String(), prefix) {
			if _, err := p.run(ctx, []string{"delete", "item", entry.ID}, nil); err != nil {
				return err
			}
		}
	}
	for _, f := range s.folders {
		if f.Name == path.String() || strings.HasPrefix(f.Name, prefix) {
			if _, err := p.run(ctx, []string{"delete", "folder", f.ID}, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
