package bitwarden

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"lazypass/internal/vault"
)

type containerRefreshRunner struct {
	fakeRunner
	commands []string
	failSync bool
}

func (r *containerRefreshRunner) Run(ctx context.Context, args []string, in io.Reader, env []string) ([]byte, error) {
	command := strings.Join(args, " ")
	r.commands = append(r.commands, command)
	if command == "create folder" {
		return []byte(`{"id":"created-folder","name":"new"}`), nil
	}
	if command == "sync" && r.failSync {
		return nil, errors.New("fixture offline")
	}
	return r.fakeRunner.Run(ctx, args, in, env)
}

func TestContainerCreationSyncsBeforeCompletion(t *testing.T) {
	for _, fail := range []bool{false, true} {
		r := &containerRefreshRunner{failSync: fail}
		p := New(t.TempDir(), "", func() (string, error) { return "fixture", nil }, r)
		err := p.CreateFolder(context.Background(), vault.Path{"Folders", "new"})
		if len(r.commands) < 2 || r.commands[len(r.commands)-1] != "sync" {
			t.Fatalf("creation completed without sync: %v", r.commands)
		}
		var warning vault.SavedWarning
		if fail && !errors.As(err, &warning) {
			t.Fatalf("successful creation reported as failed: %v", err)
		}
		if !fail && err != nil {
			t.Fatal(err)
		}
	}
}
