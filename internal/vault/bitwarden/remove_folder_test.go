package bitwarden

import (
	"context"
	"testing"

	"lazypass/internal/vault"
)

func TestRemoveCollectionDoesNotDeleteEntriesOrDescendants(t *testing.T) {
	r := &deletionRunner{groups: []collection{{ID: "parent", OrganizationID: "org", Name: "work"}, {ID: "child", OrganizationID: "org", Name: "work/nested"}}, entries: []item{{ID: "entry", OrganizationID: "org", Type: 1, CollectionIDs: []string{"parent"}}}}
	p := New(t.TempDir(), "", nil, r)
	p.session = "fixture-session"
	if err := p.RemoveFolder(context.Background(), vault.Path{"Organizations", "org", "parent"}); err != nil {
		t.Fatal(err)
	}
	if len(r.deleted) != 1 || r.deleted[0] != "delete org-collection parent --organizationid org" {
		t.Fatalf("ordinary removal deleted contents: %v", r.deleted)
	}
}

func TestMissingCollectionParentsAreNotInteractive(t *testing.T) {
	s := snapshot{organizations: []organization{{ID: "org", Name: "Team"}}, collections: []collection{{ID: "child", OrganizationID: "org", Name: "Work/Services/Production"}}}
	s.indexOrganizations()
	nodes, err := s.listOrganizations(vault.Path{"Organizations", "org"})
	if err != nil || len(nodes) != 1 || nodes[0].Name != "Work/Services/Production" {
		t.Fatalf("unexpected hierarchy: %v %v", nodes, err)
	}
}
