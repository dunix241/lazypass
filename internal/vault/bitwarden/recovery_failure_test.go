package bitwarden

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"lazypass/internal/vault"
)

type lostMoveRunner struct {
	fakeRunner
	name  string
	edits int
}

func (r *lostMoveRunner) Run(ctx context.Context, args []string, in io.Reader, env []string) ([]byte, error) {
	switch args[0] {
	case "sync":
		return nil, nil
	case "status":
		return []byte(`{"status":"unlocked","userId":"account-id","serverUrl":"https://vault.bitwarden.com"}`), nil
	case "get":
		return json.Marshal(map[string]string{"id": "source", "name": r.name})
	case "edit":
		r.edits++
		r.name = "Moved"
		return nil, errors.New("lost response")
	}
	return r.fakeRunner.Run(ctx, args, in, env)
}

func TestMoveLostResponseReconcilesAfterRestartWithoutRepeatingWrite(t *testing.T) {
	r := &lostMoveRunner{name: "Original"}
	p := New(t.TempDir(), "", nil, r)
	path := p.hierarchyOperationPath(vault.Path{"Folders", "source"}, vault.Path{"Folders"}, "Moved", true)
	job := &hierarchyJob{Version: 1, Account: transferAccount(p.Server, "account-id"), Move: true, Containers: []hierarchyContainer{{SourceID: "source", OriginalName: "Original", Name: "Moved"}}}
	if err := saveHierarchy(path, job); err != nil {
		t.Fatal(err)
	}
	if p.runHierarchyCopy(context.Background(), path, job) == nil {
		t.Fatal("lost response not surfaced")
	}
	restarted := New(p.DataDir, "", nil, r)
	loaded, err := restarted.loadHierarchy(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.runHierarchyCopy(context.Background(), path, loaded); err != nil {
		t.Fatal(err)
	}
	if r.edits != 1 || !loaded.Complete {
		t.Fatal("move replayed after lost response")
	}
}

func TestMoveExternalRenameStopsWithoutWrite(t *testing.T) {
	r := &lostMoveRunner{name: "External"}
	p := New(t.TempDir(), "", nil, r)
	path := p.hierarchyOperationPath(nil, nil, "Moved", true)
	job := &hierarchyJob{Version: 1, Account: transferAccount(p.Server, "account-id"), Move: true, Containers: []hierarchyContainer{{SourceID: "source", OriginalName: "Original", Name: "Moved"}}}
	if err := p.runHierarchyCopy(context.Background(), path, job); !errors.Is(err, vault.ErrConflict) {
		t.Fatalf("external edit: %v", err)
	}
	if r.edits != 0 || job.Failure == "" {
		t.Fatal("external rename overwritten")
	}
}
