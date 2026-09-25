package pass

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazypass/internal/vault"
)

type fakeRunner struct {
	calls []fakeCall
	fn    func(context.Context, []string, string, []string) ([]byte, error)
}
type fakeCall struct {
	args        []string
	stdin       string
	env         []string
	hasDeadline bool
}

func (f *fakeRunner) Run(ctx context.Context, name string, args []string, stdin io.Reader, env []string) ([]byte, error) {
	if name != "pass" {
		return nil, errors.New("wrong executable")
	}
	var data []byte
	if stdin != nil {
		data, _ = io.ReadAll(stdin)
	}
	_, deadline := ctx.Deadline()
	f.calls = append(f.calls, fakeCall{append([]string(nil), args...), string(data), append([]string(nil), env...), deadline})
	return f.fn(ctx, args, string(data), env)
}

func TestWriteUsesStdinAndPassArguments(t *testing.T) {
	runner := &fakeRunner{fn: func(_ context.Context, _ []string, _ string, _ []string) ([]byte, error) { return nil, nil }}
	root := t.TempDir()
	p := New(root, runner)
	secret := "not-an-argument"
	if err := p.Write(context.Background(), vault.Entry{Path: vault.Path{"Personal", "mail"}, Password: secret}); err != nil {
		t.Fatal(err)
	}
	call := runner.calls[0]
	if got := strings.Join(call.args, " "); got != "insert --multiline --force -- Personal/mail" {
		t.Fatalf("args = %q", got)
	}
	if strings.Contains(strings.Join(call.args, " "), secret) || !strings.Contains(call.stdin, secret) {
		t.Fatal("secret was not confined to stdin")
	}
	if len(call.env) != 1 || call.env[0] != "PASSWORD_STORE_DIR="+root || !call.hasDeadline {
		t.Fatalf("call = %#v", call)
	}
}

func TestReadAndSyncClassifyFailures(t *testing.T) {
	runner := &fakeRunner{fn: func(_ context.Context, args []string, _ string, _ []string) ([]byte, error) {
		if args[0] == "show" {
			return []byte("gpg: decryption failed"), errors.New("exit status 1")
		}
		return []byte("merge conflict"), errors.New("exit status 1")
	}}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "entry.gpg"), []byte("encrypted"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := New(root, runner)
	if _, err := p.Read(context.Background(), vault.Path{"entry"}); !errors.Is(err, vault.ErrLocked) {
		t.Fatalf("read error = %v", err)
	}
	if _, err := p.Sync(context.Background()); !errors.Is(err, vault.ErrConflict) {
		t.Fatalf("sync error = %v", err)
	}
	if len(runner.calls) != 2 || strings.Join(runner.calls[1].args, " ") != "git pull --rebase" {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestListOnlyIncludesSafeImmediateNodes(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(name string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("entry.gpg")
	mustWrite("plain.txt")
	if err := os.Mkdir(filepath.Join(root, "folder"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "folder", "nested.gpg"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "entry.gpg"), filepath.Join(root, "link.gpg")); err != nil {
		t.Fatal(err)
	}
	nodes, err := New(root, nil).List(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || nodes[0].Kind != vault.FolderNode || nodes[0].Name != "folder" || nodes[1].Name != "entry" {
		t.Fatalf("nodes = %#v", nodes)
	}
}

func TestRejectsNestedSymlinkPaths(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(outside, "secrets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secrets"), filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root, nil).List(context.Background(), vault.Path{"linked"}); !errors.Is(err, vault.ErrInvalidPath) {
		t.Fatalf("list symlink error = %v", err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.gpg"), filepath.Join(root, "entry.gpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root, nil).Read(context.Background(), vault.Path{"entry"}); !errors.Is(err, vault.ErrInvalidPath) {
		t.Fatalf("read symlink error = %v", err)
	}
	if err := New(root, nil).Write(context.Background(), vault.Entry{Path: vault.Path{"entry"}, Password: "secret"}); !errors.Is(err, vault.ErrInvalidPath) {
		t.Fatalf("write symlink error = %v", err)
	}
}

func TestCanceledContextDoesNotRunCommand(t *testing.T) {
	runner := &fakeRunner{fn: func(ctx context.Context, _ []string, _ string, _ []string) ([]byte, error) { return nil, ctx.Err() }}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New("/store", runner).Read(ctx, vault.Path{"entry"})
	if !errors.Is(err, vault.ErrUnavailable) {
		t.Fatalf("error = %v", err)
	}
}

func TestStoreDirectoryExpandsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got, want := New("~/.password-store", nil).StoreDirectory(), filepath.Join(home, ".password-store"); got != want {
		t.Fatalf("store directory = %q, want %q", got, want)
	}
}
