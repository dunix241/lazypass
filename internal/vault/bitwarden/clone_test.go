package bitwarden

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
)

type attachmentCloneRunner struct {
	fakeRunner
	uploaded   bool
	path       string
	failUpload bool
	creates    int
}

func (r *attachmentCloneRunner) Run(ctx context.Context, args []string, in io.Reader, env []string) ([]byte, error) {
	if len(args) > 2 && args[0] == "get" && args[1] == "item" && args[2] == "source-id" {
		return []byte(`{"attachments":[{"id":"original","fileName":"fixture.txt","size":7}]}`), nil
	}
	if len(args) > 2 && args[0] == "get" && args[1] == "item" && args[2] == "clone-id" {
		return []byte(`{"attachments":[{"id":"copied","fileName":"fixture.txt","size":7}]}`), nil
	}
	if len(args) > 1 && args[0] == "create" && args[1] == "item" {
		r.creates++
		return []byte(`{"id":"clone-id"}`), nil
	}
	if len(args) > 1 && args[0] == "get" && args[1] == "attachment" {
		r.path = args[len(args)-1]
		return nil, os.WriteFile(r.path, []byte("fixture"), 0600)
	}
	if len(args) > 1 && args[0] == "create" && args[1] == "attachment" {
		if r.failUpload {
			return nil, errors.New("fixture upload failure")
		}
		data, err := os.ReadFile(args[3])
		if err != nil {
			return nil, err
		}
		r.uploaded = string(data) == "fixture" && args[5] == "clone-id"
		return []byte(`{}`), nil
	}
	return r.fakeRunner.Run(ctx, args, in, env)
}

func TestAttachmentRecoveryReconcilesWithoutCreatingAnotherItem(t *testing.T) {
	r := &attachmentCloneRunner{failUpload: true}
	p := New(t.TempDir(), "", nil, r)
	p.session = "fixture-session"
	raw := map[string]json.RawMessage{"attachments": json.RawMessage(`[{"id":"original","fileName":"fixture.txt","size":7}]`)}
	if err := p.cloneItem(context.Background(), "source-id", raw); err != nil {
		t.Fatal(err)
	}
	if r.creates != 1 {
		t.Fatal("recovery duplicated item")
	}
	if r.uploaded {
		t.Fatal("recovery reuploaded an already present verified attachment")
	}
}

func TestCloneAttachmentUsesPrivateTemporaryFileAndCleansUp(t *testing.T) {
	r := &attachmentCloneRunner{}
	p := New(t.TempDir(), "", nil, r)
	raw := map[string]json.RawMessage{"attachments": json.RawMessage(`[{"id":"attachment-id","fileName":"fixture.txt","size":"7"}]`)}
	if err := p.cloneItem(context.Background(), "source-id", raw); err != nil {
		t.Fatal(err)
	}
	if !r.uploaded {
		t.Fatal("attachment contents not uploaded to clone")
	}
	if _, err := os.Stat(r.path); !os.IsNotExist(err) {
		t.Fatal("temporary attachment remains")
	}
}

func TestCloneRejectsUnsafeAttachmentMetadata(t *testing.T) {
	for _, value := range []string{`[{"id":"a","fileName":"../escape","size":7}]`, `[{"id":"a","fileName":"fixture","size":999999999}]`} {
		if _, err := cloneAttachments(map[string]json.RawMessage{"attachments": json.RawMessage(value)}); err == nil {
			t.Fatal("unsafe attachment accepted")
		}
	}
}

type uncertainCloneRunner struct {
	fakeRunner
	creates int
}

func (r *uncertainCloneRunner) Run(ctx context.Context, args []string, in io.Reader, env []string) ([]byte, error) {
	if args[0] == "create" && args[1] == "item" {
		r.creates++
		return nil, errors.New("lost response")
	}
	if args[0] == "sync" {
		return nil, nil
	}
	if args[0] == "list" && args[1] == "items" {
		return []byte(`[]`), nil
	}
	return r.fakeRunner.Run(ctx, args, in, env)
}

func TestUncertainCloneIsNotCreatedAgainOnRetry(t *testing.T) {
	r := &uncertainCloneRunner{}
	p := New(t.TempDir(), "", nil, r)
	for range 2 {
		raw := map[string]json.RawMessage{"name": json.RawMessage(`"fixture"`), "type": json.RawMessage(`1`)}
		if err := p.cloneItem(context.Background(), "source", raw); err == nil {
			t.Fatal("unresolved creation accepted")
		}
	}
	if r.creates != 1 {
		t.Fatal("uncertain creation duplicated")
	}
}
