package bitwarden

import (
	"testing"

	"lazypass/internal/vault"
)

func TestCollectionDeletionIncludesNestedAndSharedEntries(t *testing.T) {
	s := snapshot{organizations: []organization{{ID: "org", Name: "Team"}}, collections: []collection{
		{ID: "parent", OrganizationID: "org", Name: "work"}, {ID: "child", OrganizationID: "org", Name: "work/nested"}, {ID: "other", OrganizationID: "org", Name: "other"},
	}, excluded: []item{
		{ID: "shared", OrganizationID: "org", CollectionIDs: []string{"parent", "child", "other"}},
		{ID: "nested", OrganizationID: "org", CollectionIDs: []string{"child"}},
		{ID: "unrelated", OrganizationID: "org", CollectionIDs: []string{"other"}},
	}}
	s.indexOrganizations()
	result, err := s.collectionDeletion(vault.Path{"Organizations", "org", "parent"})
	if err != nil || result.plan.Entries != 2 || result.plan.Containers != 2 {
		t.Fatalf("wrong scope: %+v %v", result.plan, err)
	}
	first := result.plan.Token
	s.excluded = append(s.excluded, item{ID: "new", OrganizationID: "org", CollectionIDs: []string{"parent"}})
	result, err = s.collectionDeletion(vault.Path{"Organizations", "org", "parent"})
	if err != nil || result.plan.Token == first {
		t.Fatal("scope change not detected")
	}
}
