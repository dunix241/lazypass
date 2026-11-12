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

func TestFolderTransferPlanPreservesHierarchyAndRejectsConflicts(t *testing.T) {
	s := snapshot{folders: []folder{{ID: "a", Name: "Source"}, {ID: "b", Name: "Source/Child"}, {ID: "c", Name: "Target"}}}
	plan, err := s.planFolderTransfer(vault.Path{"Folders", "a"}, vault.Path{"Folders", "c"}, "Moved", true)
	if err != nil || len(plan) != 2 || plan[0].target != "Target/Moved" || plan[1].target != "Target/Moved/Child" {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	if _, err := s.planFolderTransfer(vault.Path{"Folders", "a"}, vault.Path{"Folders", "b"}, "Nested", true); err == nil {
		t.Fatal("descendant destination accepted")
	}
	if _, err := s.planFolderTransfer(vault.Path{"Folders", "a"}, vault.Path{"Folders"}, "Source", false); err != vault.ErrConflict {
		t.Fatal("conflict accepted")
	}
}

func TestTransferHierarchyUsesNamesInsteadOfIDPrefixes(t *testing.T) {
	p := New(t.TempDir(), "", nil, nil)
	s := snapshot{folders: []folder{{ID: "parent-id", Name: "Source"}, {ID: "independent-id", Name: "Source/Child"}, {ID: "other-id", Name: "Other"}}}
	p.browser.Store(&s)
	parent := vault.Node{Path: vault.Path{"Folders", "parent-id"}, Kind: vault.FolderNode}
	child := vault.Node{Path: vault.Path{"Folders", "independent-id"}, Kind: vault.FolderNode}
	if !p.TransferContains(parent, child) {
		t.Fatal("logical nested folder not recognized")
	}
	child.Kind = vault.EntryNode
	child.Path = append(child.Path, "item-id")
	if !p.TransferContains(parent, child) {
		t.Fatal("nested item not recognized")
	}
	if p.TransferContains(parent, parent) {
		t.Fatal("folder contains itself")
	}
	child.Path = vault.Path{"Folders", "other-id", "item-id"}
	if p.TransferContains(parent, child) {
		t.Fatal("unrelated folder matched")
	}
}

type containerRunner struct {
	fakeRunner
	edits map[string]map[string]json.RawMessage
}

func (r *containerRunner) Run(ctx context.Context, args []string, in io.Reader, env []string) ([]byte, error) {
	if args[0] == "status" {
		return []byte(`{"status":"unlocked","userId":"account-id","serverUrl":"https://vault.bitwarden.com"}`), nil
	}
	if len(args) > 2 && args[0] == "get" && args[1] == "org-collection" {
		name := "Source"
		if args[2] == "b" {
			name = "Source/Child"
		}
		return []byte(`{"id":"` + args[2] + `","name":"` + name + `","groups":[{"id":"keep-group"}],"users":[{"id":"keep-user"}],"unknown":true}`), nil
	}
	if len(args) > 2 && args[0] == "edit" {
		encoded, err := io.ReadAll(in)
		if err != nil {
			return nil, err
		}
		data, err := base64.StdEncoding.DecodeString(string(encoded))
		if err != nil {
			return nil, err
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
		if r.edits == nil {
			r.edits = map[string]map[string]json.RawMessage{}
		}
		r.edits[strings.Join(args, " ")] = raw
		return []byte(`{}`), nil
	}
	return r.fakeRunner.Run(ctx, args, in, env)
}

func TestCollectionMovePreservesAdministrativeSettings(t *testing.T) {
	r := &containerRunner{}
	p := New(t.TempDir(), "", nil, r)
	s := snapshot{organizations: []organization{{ID: "org"}}, collections: []collection{{ID: "a", OrganizationID: "org", Name: "Source"}, {ID: "b", OrganizationID: "org", Name: "Source/Child"}, {ID: "c", OrganizationID: "org", Name: "Target"}}}
	s.indexOrganizations()
	p.metadata = &s
	if err := p.TransferFolder(context.Background(), vault.Path{"Organizations", "org", "a"}, vault.Path{"Organizations", "org", "c"}, "Moved", true); err != nil {
		t.Fatal(err)
	}
	if len(r.edits) != 2 {
		t.Fatal("hierarchy not moved")
	}
	for _, raw := range r.edits {
		if len(raw["users"]) == 0 || len(raw["groups"]) == 0 || string(raw["unknown"]) != "true" {
			t.Fatal("access/settings lost")
		}
	}
	if _, err := s.planFolderTransfer(vault.Path{"Organizations", "org", "a"}, vault.Path{"Organizations", "org"}, "Copy", false); err != nil {
		t.Fatal("collection copy hierarchy rejected", err)
	}
}
