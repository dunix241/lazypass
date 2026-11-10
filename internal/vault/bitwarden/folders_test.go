package bitwarden

import (
	"context"
	"errors"
	"testing"

	"lazypass/internal/vault"
)

func TestSharedEntryHasIndependentFolderAndCollectionViews(t *testing.T) {
	shared := item{ID: "shared", Name: "same/name", Type: 1, OrganizationID: "org", FolderID: "folder", CollectionIDs: []string{"collection"}}
	s := snapshot{folders: []folder{{ID: "folder", Name: "Work"}}, organizations: []organization{{ID: "org", Name: "Team"}}, collections: []collection{{ID: "collection", OrganizationID: "org", Name: "Services"}}, excluded: []item{shared}, items: []item{{ID: "personal", Name: "personal", Type: 1}}}
	s.indexOrganizations()
	nodes, err := s.listFolders(vault.Path{"Folders", "folder"})
	if err != nil || len(nodes) != 1 || !nodes[0].Shared || nodes[0].Name != "same/name" {
		t.Fatalf("shared folder membership: %v %v", nodes, err)
	}
	a, err := s.resolve(nodes[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.resolve(vault.Path{"Organizations", "org", "collection", "shared"})
	if err != nil || a.ID != b.ID {
		t.Fatal("views resolve different objects")
	}
	unassigned, err := s.listFolders(vault.Path{"Folders", noFolderID})
	if err != nil || len(unassigned) != 1 || unassigned[0].Name != "personal" {
		t.Fatal("No Folder assignment incorrect")
	}
}

func TestFolderIDCreationRejectsDuplicateEntryNames(t *testing.T) {
	r := &fakeRunner{}
	p := New(t.TempDir(), "", func() (string, error) { return "fixture", nil }, r)
	err := p.Create(context.Background(), vault.Entry{
		Path: vault.Path{"Folders", "folder-id", "world"},
		Type: vault.LoginEntry,
	})
	if !errors.Is(err, vault.ErrConflict) {
		t.Fatalf("duplicate accepted: %v", err)
	}
}

func TestUnknownFolderReferenceIsNotAnEmptyFolder(t *testing.T) {
	s := snapshot{}
	if _, err := s.listFolders(vault.Path{"Folders", "missing"}); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("unknown folder accepted: %v", err)
	}
	if _, err := s.resolveFolderItem(vault.Path{"Folders", "missing", "item"}); !errors.Is(err, vault.ErrNotFound) {
		t.Fatalf("unknown item folder accepted: %v", err)
	}
}
