package vault

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/vault"
)

func TestOrganizationNameEditingDoesNotChangeLocation(t *testing.T) {
	s := NewStoreForm(StoreDependencies{})
	s.open, s.locationOnly = true, true
	s.location = vault.Path{"Organizations", "organization-id", "collection-id"}
	s.storePath = s.location.String() + "/"
	s.Paste("entry")
	storeKey(s, tcell.KeyCtrlU, 0)
	if s.entryName != "" || s.storePath != "Organizations/organization-id/collection-id/" {
		t.Fatal("name editing changed location")
	}
	storeKey(s, tcell.KeyBackspace2, 0)
	if len(s.location) != 3 {
		t.Fatal("backspace removed selected collection")
	}
}

func TestLocationLabelUsesNames(t *testing.T) {
	v := &View{locationLabels: map[string]string{"Organizations/org": "Team", "Organizations/org/group": "Services"}}
	if got := v.LocationLabel(vault.Path{"Organizations", "org", "group"}); got != "/Organizations/Team/Services" {
		t.Fatalf("breadcrumb: %s", got)
	}
}

func TestInvalidOrganizationDestinationDoesNotOpenDetails(t *testing.T) {
	s := NewStoreForm(StoreDependencies{})
	s.open, s.locationOnly, s.manual = true, true, true
	s.location = vault.Path{"Organizations", "org", "collection"}
	s.entryName = "test/test"
	s.storePath = s.location.String() + "/" + s.entryName
	storeKey(s, tcell.KeyEnter, 0)
	if s.editingValue {
		t.Fatal("invalid destination opened details")
	}
	s.location = vault.Path{"Organizations", "org"}
	s.storePath = s.location.String() + "/" + s.entryName
	storeKey(s, tcell.KeyEnter, 0)
	if s.editingValue || len(s.location) != 2 {
		t.Fatal("typed destination selected an unrelated collection")
	}
}
