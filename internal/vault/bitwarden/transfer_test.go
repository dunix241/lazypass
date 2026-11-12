package bitwarden

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"lazypass/internal/vault"
	"strings"
	"testing"
)

type transferRunner struct {
	fakeRunner
	command string
	payload map[string]json.RawMessage
}

func (r *transferRunner) Run(ctx context.Context, args []string, in io.Reader, env []string) ([]byte, error) {
	cmd := strings.Join(args, " ")
	if cmd == "get item source" {
		return []byte(`{"id":"source","type":1,"name":"Original","folderId":"a","organizationId":null,"login":{"password":"fixture","totp":"JBSWY3DPEHPK3PXP","fido2Credentials":[{"credentialId":"fixture"}]},"fields":[{"name":"unknown","value":"retained"}],"unknownFuture":{"preserve":true}}`), nil
	}
	if cmd == "create item" || cmd == "edit item source" {
		r.command = cmd
		encoded, err := io.ReadAll(in)
		if err != nil {
			return nil, err
		}
		data, err := base64.StdEncoding.DecodeString(string(encoded))
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(data, &r.payload)
		return []byte(`{"id":"result"}`), err
	}
	return r.fakeRunner.Run(ctx, args, in, env)
}

func TestNativeTransferPreservesFullItemAndMoveIdentity(t *testing.T) {
	for _, move := range []bool{false, true} {
		r := &transferRunner{}
		p := New(t.TempDir(), "", func() (string, error) { return "fixture", nil }, r)
		p.metadata = &snapshot{folders: []folder{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}, items: []item{{ID: "source", Name: "Original", FolderID: "a", Type: 1}}}
		if err := p.Transfer(context.Background(), vault.Path{"Folders", "a", "source"}, vault.Path{"Folders", "b"}, "Renamed", move); err != nil {
			t.Fatal(err)
		}
		if string(r.payload["unknownFuture"]) != `{"preserve":true}` || !strings.Contains(string(r.payload["login"]), "fido2Credentials") || string(r.payload["folderId"]) != `"b"` {
			t.Fatal("native fields lost")
		}
		if move && (r.command != "edit item source" || string(r.payload["id"]) != `"source"`) {
			t.Fatal("move replaced identity")
		}
		if !move && (r.command != "create item" || r.payload["id"] != nil) {
			t.Fatal("clone retained source ID")
		}
	}
}

func TestTransferInfersOwnershipFromDestination(t *testing.T) {
	s := snapshot{
		folders:       []folder{{ID: "folder", Name: "Personal"}},
		items:         []item{{ID: "personal", Name: "Fixture", Type: 1, FolderID: "folder"}},
		organizations: []organization{{ID: "org"}, {ID: "other"}},
		collections:   []collection{{ID: "collection", OrganizationID: "org", Name: "Collection"}, {ID: "target", OrganizationID: "other", Name: "Target"}},
		excluded:      []item{{ID: "shared", Name: "Shared", Type: 1, OrganizationID: "org", CollectionIDs: []string{"collection"}}},
	}
	s.indexOrganizations()
	personal := vault.Path{"Folders", "folder", "personal"}
	shared := vault.Path{"Organizations", "org", "collection", "shared"}
	for _, move := range []bool{false, true} {
		if _, _, err := s.checkTransfer(personal, vault.Path{"Organizations", "org", "collection"}, "Fixture", move); err != nil {
			t.Fatalf("personal to organization: %v", err)
		}
	}
	if _, _, err := s.checkTransfer(shared, vault.Path{"Folders", "folder"}, "Shared copy", false); err != nil {
		t.Fatal("organization to personal copy rejected", err)
	}
	if _, _, err := s.checkTransfer(shared, vault.Path{"Organizations", "other", "target"}, "Shared copy", false); err != nil {
		t.Fatal("cross-organization copy rejected", err)
	}
	if _, _, err := s.checkTransfer(shared, vault.Path{"Organizations", "other", "target"}, "Shared", true); err == nil {
		t.Fatal("unsupported cross-organization move accepted")
	}
}

func TestCollectionMovePreservesOtherMembershipsAndOwnership(t *testing.T) {
	s := snapshot{organizations: []organization{{ID: "org"}}, collections: []collection{{ID: "a", OrganizationID: "org"}, {ID: "b", OrganizationID: "org"}, {ID: "c", OrganizationID: "org"}}, excluded: []item{{ID: "source", Name: "Original", Type: 1, OrganizationID: "org", CollectionIDs: []string{"a", "c"}}}}
	s.indexOrganizations()
	_, target, err := s.checkTransfer(vault.Path{"Organizations", "org", "a", "source"}, vault.Path{"Organizations", "org", "b"}, "Original", true)
	if err != nil || target.collection.ID != "b" {
		t.Fatal(err)
	}
	if _, _, err := s.checkTransfer(vault.Path{"Organizations", "org", "a", "source"}, vault.Path{"Organizations", "org", "b"}, "Renamed", true); err == nil {
		t.Fatal("membership move allowed global rename")
	}
}
