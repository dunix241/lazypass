package bitwarden

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"lazypass/internal/vault"
)

type deletionRunner struct {
	fakeRunner
	groups  []collection
	entries []item
	deleted []string
	fail    string
}

func (r *deletionRunner) Run(ctx context.Context, args []string, in io.Reader, env []string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	command := strings.Join(args, " ")
	if strings.HasPrefix(command, "delete ") {
		if command == r.fail {
			return nil, errors.New("fixture failure")
		}
		r.deleted = append(r.deleted, command)
		return []byte(`{}`), nil
	}
	switch command {
	case "list organizations":
		return []byte(`[{"id":"org","name":"Team"}]`), nil
	case "list collections":
		return json.Marshal(r.groups)
	case "list items":
		return json.Marshal(r.entries)
	case "list folders":
		return []byte(`[]`), nil
	case "sync":
		return []byte(`{}`), nil
	}
	return r.fakeRunner.Run(ctx, args, in, env)
}

func TestDeleteExecutionGuardsAndPartialFailures(t *testing.T) {
	for _, scenario := range []string{"success", "changed", "denied", "cancelled", "partial"} {
		t.Run(scenario, func(t *testing.T) {
			r := &deletionRunner{groups: []collection{{ID: "z-parent", OrganizationID: "org", Name: "work"}, {ID: "a-child", OrganizationID: "org", Name: "work/child"}}, entries: []item{{ID: "a", Name: "a", Type: 1, OrganizationID: "org", CollectionIDs: []string{"z-parent", "a-child"}}, {ID: "b", Name: "b", Type: 1, OrganizationID: "org", CollectionIDs: []string{"a-child"}}}}
			p := New(t.TempDir(), "", nil, r)
			p.session = "fixture-session"
			plan, err := p.PlanDelete(context.Background(), vault.Path{"Organizations", "org", "z-parent"})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			switch scenario {
			case "changed":
				r.entries = append(r.entries, item{ID: "new", Name: "new", Type: 1, OrganizationID: "org", CollectionIDs: []string{"z-parent"}})
			case "denied":
				denied := true
				r.groups[0].ReadOnly = &denied
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "partial":
				r.fail = "delete item b"
			}
			err = p.ExecuteDelete(ctx, plan)
			if scenario == "success" {
				if err != nil {
					t.Fatal(err)
				}
				want := []string{"delete item a", "delete item b", "delete org-collection a-child --organizationid org", "delete org-collection z-parent --organizationid org"}
				if strings.Join(r.deleted, "\n") != strings.Join(want, "\n") {
					t.Fatalf("wrong deletion order: %v", r.deleted)
				}
			} else {
				if err == nil {
					t.Fatal("guard accepted destructive execution")
				}
				want := 0
				if scenario == "partial" {
					want = 1
				}
				if len(r.deleted) != want {
					t.Fatalf("unexpected destructive calls: %v", r.deleted)
				}
			}
			if p.metadata != nil {
				t.Fatal("deletion retained stale metadata")
			}
		})
	}
}
