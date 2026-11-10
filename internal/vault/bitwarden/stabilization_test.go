package bitwarden

import (
	"context"
	"errors"
	"io"
	"testing"

	"lazypass/internal/vault"
)

func TestRealCollectionIsParentOfNestedCollection(t *testing.T) {
	s := snapshot{
		organizations: []organization{{ID: "org", Name: "Team"}},
		collections: []collection{
			{ID: "parent", OrganizationID: "org", Name: "infra"},
			{ID: "child", OrganizationID: "org", Name: "infra/prod"},
			{ID: "leaf", OrganizationID: "org", Name: "infra/prod/api"},
		},
	}
	s.indexOrganizations()
	root, _ := s.listOrganizations(vault.Path{"Organizations", "org"})
	if len(root) != 1 || root[0].Path[2] != "parent" {
		t.Fatalf("duplicate virtual parent: %v", root)
	}
	children, _ := s.listOrganizations(root[0].Path)
	if len(children) != 1 || children[0].Path[2] != "child" {
		t.Fatalf("nested child missing: %v", children)
	}
	leaves, _ := s.listOrganizations(children[0].Path)
	if len(leaves) != 1 || leaves[0].Path[2] != "leaf" {
		t.Fatalf("nested leaf missing: %v", leaves)
	}
}

type failedSyncRunner struct{ organizationRunner }

func (r *failedSyncRunner) Run(ctx context.Context, args []string, input io.Reader, env []string) ([]byte, error) {
	if len(args) == 1 && args[0] == "sync" {
		return nil, errors.New("fixture connection lost")
	}
	return r.organizationRunner.Run(ctx, args, input, env)
}

func TestSavedEntryIsNotReportedAsFailedWhenSyncFails(t *testing.T) {
	r := &failedSyncRunner{}
	p := New(t.TempDir(), "", func() (string, error) { return "fixture", nil }, r)
	err := p.Create(context.Background(), vault.Entry{Path: vault.Path{"Organizations", "org-id", "group-a", "new entry"}, Type: vault.NoteEntry})
	var warning vault.SavedWarning
	if !errors.As(err, &warning) || r.created == nil {
		t.Fatalf("saved write not distinguished from failed write: %v", err)
	}
}

func TestCreationByCollectionNameAndAmbiguousNames(t *testing.T) {
	s := snapshot{organizations: []organization{{ID: "org", Name: "Team"}}, collections: []collection{
		{ID: "one", OrganizationID: "org", Name: "infra/prod"},
	}}
	s.indexOrganizations()
	plan, err := s.planCreation(vault.Path{"Organizations", "org", "infra", "stage", "entry"})
	if err != nil || plan.collectionName != "infra/stage" {
		t.Fatalf("virtual destination: %+v %v", plan, err)
	}
	s.collections = append(s.collections, collection{ID: "two", OrganizationID: "org", Name: "infra/prod"})
	s.indexOrganizations()
	if _, err := s.planCreation(vault.Path{"Organizations", "org", "infra", "prod", "entry"}); !errors.Is(err, vault.ErrConflict) {
		t.Fatalf("ambiguous name accepted: %v", err)
	}
	if _, err := s.planCreation(vault.Path{"Organizations", "org", "one", "entry"}); err != nil {
		t.Fatalf("explicit ID rejected: %v", err)
	}
}
