package bitwarden

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"lazypass/internal/vault"
)

type organizationRunner struct {
	fakeRunner
	created map[string]any
}

func (r *organizationRunner) Run(ctx context.Context, args []string, input io.Reader, env []string) ([]byte, error) {
	switch strings.Join(args, " ") {
	case "create item":
		encoded, err := io.ReadAll(input)
		if err != nil {
			return nil, err
		}
		data, err := base64.StdEncoding.DecodeString(string(encoded))
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(data, &r.created)
		return []byte(`{}`), err
	case "list folders":
		return []byte(`[{"id":null,"name":"No Folder"}]`), nil
	case "list organizations":
		return []byte(`[{"id":"org-id","name":"Team / Production"}]`), nil
	case "list collections":
		return []byte(`[{"id":"group-a","organizationId":"org-id","name":"Services / API"},{"id":"group-b","organizationId":"org-id","name":"Services / API"}]`), nil
	case "list items":
		return []byte(`[{"id":"item-id","name":"duplicate/name","type":1,"organizationId":"org-id","collectionIds":["group-a","group-b"],"login":{"password":"not-cached"}},{"id":"other-id","name":"duplicate/name","type":2,"organizationId":"org-id","collectionIds":["group-a"]}]`), nil
	case "get item item-id":
		r.gets++
		return []byte(`{"id":"item-id","type":1,"name":"duplicate/name","organizationId":"org-id","collectionIds":["group-a","group-b"],"login":{"password":"fixture"},"fields":[{"name":"custom","value":"kept"}]}`), nil
	}
	return r.fakeRunner.Run(ctx, args, input, env)
}

func TestOrganizationCreateAndDeniedMutation(t *testing.T) {
	r := &organizationRunner{}
	p := New(t.TempDir(), "", func() (string, error) { return "fixture", nil }, r)
	ctx := context.Background()
	entry := vault.Entry{Path: vault.Path{"Organizations", "org-id", "group-a", "new entry"}, Type: vault.NoteEntry, Notes: "public fixture"}
	if err := p.Create(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if r.created["organizationId"] != "org-id" || r.created["name"] != "new entry" {
		t.Fatalf("incorrect creation target")
	}
	if len(r.created["collectionIds"].([]any)) != 1 {
		t.Fatal("collection missing")
	}
	if _, err := p.List(ctx, nil); err != nil {
		t.Fatal(err)
	}
	denied := false
	s := p.metadata
	path := vault.Path{"Organizations", "org-id", "group-a", "item-id"}
	item := s.organizationItems[path.String()]
	item.Edit = &denied
	s.organizationItems[path.String()] = item
	if err := p.Delete(ctx, path); err == nil {
		t.Fatal("known read-only entry deleted")
	}
}

func TestOrganizationIndexKeepsEmptyLocationsAndIsolatesReturnedPaths(t *testing.T) {
	s := snapshot{
		organizations: []organization{{ID: "empty-org", Name: "Empty"}, {ID: "org", Name: "Team"}},
		collections:   []collection{{ID: "empty-group", OrganizationID: "org", Name: "Empty collection"}},
		excluded:      []item{{ID: "unfiled", OrganizationID: "org", Name: "Shared", Type: 1}},
	}
	s.indexOrganizations()
	nodes, err := s.listOrganizations(vault.Path{"Organizations"})
	if err != nil || len(nodes) != 2 {
		t.Fatalf("empty organization missing: %v %v", nodes, err)
	}
	groups, err := s.listOrganizations(vault.Path{"Organizations", "org"})
	if err != nil || len(groups) != 2 {
		t.Fatalf("empty collection or uncollected missing: %v %v", groups, err)
	}
	entries, err := s.listOrganizations(vault.Path{"Organizations", "org", "uncollected"})
	if err != nil || len(entries) != 1 {
		t.Fatalf("uncollected entry missing: %v %v", entries, err)
	}
	entries[0].Path[1] = "mutated"
	if _, err := s.resolveOrganizationItem(vault.Path{"Organizations", "org", "uncollected", "unfiled"}); err != nil {
		t.Fatal("caller corrupted cached identity")
	}
	again, _ := s.listOrganizations(vault.Path{"Organizations", "org", "uncollected"})
	if again[0].Path[1] != "org" {
		t.Fatal("cached path mutated")
	}
	if _, err := s.listOrganizations(vault.Path{"Organizations", "unknown"}); err != vault.ErrNotFound {
		t.Fatal("unknown location accepted")
	}
}

func TestOrganizationBrowsingAndPreservingEdit(t *testing.T) {
	r := &organizationRunner{}
	p := New(t.TempDir(), "", func() (string, error) { return "fixture", nil }, r)
	ctx := context.Background()
	nodes, err := p.List(ctx, nil)
	if err != nil || len(nodes) != 2 {
		t.Fatalf("root %v %v", nodes, err)
	}
	personal, err := p.List(ctx, vault.Path{"Personal"})
	if err != nil || len(personal) != 0 {
		t.Fatalf("virtual folder leaked: %v %v", personal, err)
	}
	orgs, err := p.List(ctx, vault.Path{"Organizations"})
	if err != nil || len(orgs) != 1 || orgs[0].Name != "Team / Production" {
		t.Fatalf("organizations %v %v", orgs, err)
	}
	groups, err := p.List(ctx, orgs[0].Path)
	if err != nil || len(groups) != 2 || groups[0].Path.String() == groups[1].Path.String() {
		t.Fatalf("collection identities %v %v", groups, err)
	}
	entries, err := p.List(ctx, groups[0].Path)
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries %v %v", entries, err)
	}
	if !entries[0].Shared {
		t.Fatal("shared entry not identified")
	}
	for _, entry := range p.metadata.excluded {
		if entry.Login.Password != "" {
			t.Fatal("metadata retained password")
		}
	}
	path := vault.Path{"Organizations", "org-id", "group-b", "item-id"}
	entry, err := p.Read(ctx, path)
	if err != nil || entry.Password != "fixture" {
		t.Fatalf("read by ID: %v", err)
	}
	entry.Password = "changed"
	if err := p.Write(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if r.edited["fields"] == nil {
		t.Fatal("raw fields lost")
	}
	if r.edited["organizationId"] != "org-id" || len(r.edited["collectionIds"].([]any)) != 2 {
		t.Fatal("organization membership lost during edit")
	}
	if err := p.DeleteFolder(ctx, groups[0].Path); err != vault.ErrUnsupported {
		t.Fatalf("collection delete allowed: %v", err)
	}
}
