package bitwarden

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"lazypass/internal/vault"
)

type fakeRunner struct {
	itemType int
	gets     int
	calls    int
	edited   map[string]any
	unlocks  int
}

func (r *fakeRunner) Run(_ context.Context, args []string, input io.Reader, env []string) ([]byte, error) {
	r.calls++
	switch strings.Join(args, " ") {
	case "status":
		state := "locked"
		for _, value := range env {
			if value == "BW_SESSION=fixture-session" {
				state = "unlocked"
			}
		}
		return []byte(`{"status":"` + state + `","serverUrl":"https://vault.bitwarden.com"}`), nil
	case "unlock --passwordenv LAZYPASS_BW_PASSWORD --raw":
		r.unlocks++
		return []byte("fixture-session\n"), nil
	case "list folders":
		return []byte(`[{"id":"folder-id","name":"hello"}]`), nil
	case "list organizations", "list collections":
		return []byte(`[]`), nil
	case "list items":
		if r.itemType != 0 {
			return []byte(`[{"id":"item-id","name":"world","folderId":"folder-id","type":3}]`), nil
		}
		return []byte(`[{"id":"item-id","name":"world","folderId":"folder-id","type":1,"login":{"username":"user","password":"fixture"}}]`), nil
	case "get item item-id":
		r.gets++
		if r.itemType != 0 {
			return []byte(`{"id":"item-id","type":3,"name":"world","card":{"number":"4111111111111111"}}`), nil
		}
		return []byte(`{"id":"item-id","type":1,"name":"world","login":{"password":"fixture","totp":"preserved","uris":[{"uri":"https://one"},{"uri":"https://two"}]},"fields":[{"name":"custom","value":"kept"}]}`), nil
	case "get totp item-id":
		return []byte("123456\n"), nil
	case "edit item item-id":
		encoded, _ := io.ReadAll(input)
		data, err := base64.StdEncoding.DecodeString(string(encoded))
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(data, &r.edited)
		return []byte(`{}`), err
	default:
		return []byte(`{}`), nil
	}
}

func TestBuiltInPromptChecksStatusBeforeUnlockAndIsNotReused(t *testing.T) {
	r := &fakeRunner{}
	p := New(t.TempDir(), "", nil, r)
	ctx := vault.WithPassphrasePrompt(context.Background(), func(context.Context) (string, error) {
		if r.calls != 1 {
			t.Error("prompt must follow status and precede entry loading")
		}
		return "fixture", nil
	})
	if _, err := p.List(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if r.unlocks != 1 {
		t.Fatal("unlock missing")
	}
	ctx = vault.WithPassphrasePrompt(context.Background(), func(context.Context) (string, error) { t.Error("unlocked session prompted again"); return "", nil })
	if _, err := p.List(ctx, nil); err != nil {
		t.Fatal(err)
	}
}

type signedOutRunner struct{ fakeRunner }

func (r *signedOutRunner) Run(ctx context.Context, args []string, input io.Reader, env []string) ([]byte, error) {
	if strings.Join(args, " ") == "status" {
		return []byte(`{"status":"unauthenticated","serverUrl":"https://vault.bitwarden.com"}`), nil
	}
	return r.fakeRunner.Run(ctx, args, input, env)
}

func TestSignedOutNeverRequestsPassphrase(t *testing.T) {
	p := New(t.TempDir(), "", func() (string, error) { t.Error("signed-out account prompted"); return "", nil }, &signedOutRunner{})
	for range 2 {
		_, err := p.List(context.Background(), nil)
		if !errors.Is(err, vault.ErrUninitialized) {
			t.Fatalf("wrong state: %v", err)
		}
		var diagnostic vault.Diagnostic
		if !errors.As(err, &diagnostic) || !strings.Contains(diagnostic.UserMessage(), "bw login") || !strings.Contains(diagnostic.UserMessage(), p.DataDir) {
			t.Fatal("missing relogin guidance")
		}
	}
}

func TestTypedReadUsesOneCLIGet(t *testing.T) {
	r := &fakeRunner{itemType: 3}
	p := New(t.TempDir(), "", func() (string, error) { return "fixture", nil }, r)
	entry, err := p.Read(context.Background(), vault.Path{"hello", "world"})
	if err != nil || entry.Type != vault.CardEntry || entry.Fields["number"] != "4111111111111111" {
		t.Fatalf("typed read: %v", err)
	}
	if r.gets != 1 {
		t.Fatalf("typed read launched %d CLI get commands", r.gets)
	}
}

func TestBrowsingUsesMetadataCacheWithoutPasswords(t *testing.T) {
	r := &fakeRunner{}
	p := New(t.TempDir(), "", func() (string, error) { return "fixture", nil }, r)
	ctx := context.Background()
	if _, err := p.List(ctx, nil); err != nil {
		t.Fatal(err)
	}
	calls := r.calls
	for range 20 {
		if _, err := p.List(ctx, vault.Path{"hello"}); err != nil {
			t.Fatal(err)
		}
	}
	if r.calls != calls {
		t.Fatalf("browsing launched %d additional commands", r.calls-calls)
	}
	if p.metadata == nil || p.metadata.items[0].Login.Password != "" {
		t.Fatal("metadata cache contains passwords")
	}
	p.Lock()
	if p.metadata != nil {
		t.Fatal("locking retained metadata")
	}
}

func TestSessionFolderMappingAndPreservingEdit(t *testing.T) {
	r := &fakeRunner{}
	p := New(t.TempDir(), "", func() (string, error) { return "public-fixture", nil }, r)
	ctx := context.Background()
	if _, err := p.List(vault.WithoutPrompt(ctx), nil); err != vault.ErrPromptRequired {
		t.Fatalf("probe=%v", err)
	}
	nodes, err := p.List(ctx, nil)
	if err != nil || len(nodes) != 2 || nodes[0].Name != "Folders" || nodes[1].Name != "Organizations" {
		t.Fatalf("nodes=%v err=%v", nodes, err)
	}
	nodes, err = p.List(ctx, vault.Path{"hello"})
	if err != nil || len(nodes) != 1 || nodes[0].Name != "world" {
		t.Fatal("folder mapping failed")
	}
	entry, err := p.Read(ctx, vault.Path{"hello", "world"})
	if err != nil || entry.Password != "fixture" {
		t.Fatal("read failed")
	}
	entry.Password = "updated"
	if err := p.Write(ctx, entry); err != nil {
		t.Fatal(err)
	}
	login := r.edited["login"].(map[string]any)
	if login["password"] != "updated" || login["totp"] != "preserved" || len(login["uris"].([]any)) != 2 || r.edited["fields"] == nil {
		t.Fatal("edit discarded metadata")
	}
	if r.unlocks != 1 {
		t.Fatal("session not reused")
	}
	if err := p.Create(ctx, entry); err != vault.ErrConflict {
		t.Fatal("duplicate allowed")
	}
	p.Lock()
	if p.session != "" {
		t.Fatal("session retained")
	}
}

func TestDuplicateNamesRejectAmbiguousOperation(t *testing.T) {
	s := snapshot{items: []item{{ID: "one", Name: "same"}, {ID: "two", Name: "same"}}}
	if _, err := s.resolve(vault.Path{"same"}); err != vault.ErrConflict {
		t.Fatal("ambiguous item resolved")
	}
}
