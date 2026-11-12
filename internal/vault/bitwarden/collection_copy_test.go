package bitwarden

import (
	"context"
	"encoding/json"
	"io"
	"testing"
)

type currentMemberRunner struct {
	fakeRunner
}

func (r *currentMemberRunner) Run(ctx context.Context, args []string, in io.Reader, env []string) ([]byte, error) {
	if len(args) > 2 && args[0] == "get" && args[1] == "folder" && args[2] == "confirmed" {
		return []byte(`{"id":"confirmed","name":"Copy"}`), nil
	}
	if args[0] == "status" {
		return []byte(`{"status":"unlocked","userId":"account-id","serverUrl":"https://vault.bitwarden.com"}`), nil
	}
	return r.fakeRunner.Run(ctx, args, in, env)
}

func TestCollectionCopyDoesNotAssignSharingPermissions(t *testing.T) {
	payload := collectionClonePayload(containerTransfer{organizationID: "org", target: "Copy"})
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct{ Users, Groups []json.RawMessage }
	if json.Unmarshal(data, &decoded) != nil || len(decoded.Users) != 0 || len(decoded.Groups) != 0 {
		t.Fatal("unexpected sharing grants")
	}
}
