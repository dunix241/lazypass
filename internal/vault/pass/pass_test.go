package pass

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

type fakeExitError struct{ code int }

func (e fakeExitError) Error() string { return "exit status" }
func (e fakeExitError) ExitCode() int { return e.code }

func TestExecRunnerPreservesTerminalAndExplicitSecretInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture requires Unix")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pass"), []byte("#!/bin/sh\nread value\nprintf '%s' \"$value\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	runner := ExecRunner{Terminal: strings.NewReader("terminal\n")}
	output, err := runner.Run(context.Background(), "pass", nil, nil, nil)
	if err != nil || string(output) != "terminal" {
		t.Fatalf("read-only command input = %q, %v", output, err)
	}
	output, err = runner.Run(context.Background(), "pass", nil, strings.NewReader("secret-input\n"), nil)
	if err != nil || string(output) != "secret-input" {
		t.Fatalf("write command did not use its provided stdin: %v", err)
	}
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

func TestMissingGPGKeyIsNotClassifiedAsMissingEntry(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "entry.gpg"), []byte("encrypted"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{fn: func(context.Context, []string, string, []string) ([]byte, error) {
		return []byte("gpg: secret key not found"), errors.New("exit status 1")
	}}
	if _, err := New(root, runner).Read(context.Background(), vault.Path{"entry"}); !errors.Is(err, vault.ErrLocked) {
		t.Fatalf("missing GPG key classified as %v", err)
	}
}

func TestPinentryFailureHasSafeDebugCategory(t *testing.T) {
	state := t.TempDir()
	t.Setenv("LAZYPASS_DEBUG", "1")
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("GPG_TTY", "/dev/pts/sensitive-metadata")
	t.Setenv("SSH_TTY", "/dev/pts/other")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "entry.gpg"), []byte("encrypted"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{fn: func(context.Context, []string, string, []string) ([]byte, error) {
		return []byte("gpg: No pinentry; secret-value"), fakeExitError{code: 2}
	}}
	if _, err := New(root, runner).Read(context.Background(), vault.Path{"entry"}); !errors.Is(err, vault.ErrLocked) {
		t.Fatalf("pinentry error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(state, "lazypass", "debug.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `operation="pass show" cause="pinentry" exit=2`) || !strings.Contains(string(data), "gpg_tty=true ssh=true") || strings.Contains(string(data), "secret-value") || strings.Contains(string(data), "sensitive-metadata") {
		t.Fatalf("unsafe pinentry diagnostics: %q", data)
	}
}

func TestGPGDiagnosticsAreSpecificAndSecretFree(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "entry.gpg"), []byte("encrypted"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		stderr, reason, notice string
	}{
		{"gpg: decryption failed: No secret key", "no matching secret key", "GPG: missing secret key"},
		{"gpg: public key decryption failed: No pinentry", "cannot start pinentry", "GPG: pinentry unavailable"},
		{"gpg: public key decryption failed: Inappropriate ioctl for device", "cannot access a terminal", "GPG: no terminal for pinentry"},
		{"gpg: problem with the agent: No agent running", "cannot contact gpg-agent", "GPG: agent unavailable"},
		{"gpg: bad passphrase", "rejected the passphrase", "GPG: passphrase rejected"},
		{"gpg: decryption failed", "could not decrypt this entry", "GPG: decryption failed"},
		{"gpg: unrecognized error", "cause not identified", "GPG: unknown failure; run pass show"},
	} {
		t.Run(test.reason, func(t *testing.T) {
			runner := &fakeRunner{fn: func(context.Context, []string, string, []string) ([]byte, error) {
				return []byte(test.stderr + " secret-value"), fakeExitError{code: 2}
			}}
			_, err := New(root, runner).Read(context.Background(), vault.Path{"entry"})
			var failure *GPGFailure
			if !errors.As(err, &failure) || !errors.Is(err, vault.ErrLocked) {
				t.Fatalf("GPG error not preserved: %v", err)
			}
			if failure.ExitCode != 2 || !strings.Contains(failure.Error(), test.reason) || failure.UserMessage() != test.notice || strings.Contains(failure.Error(), "secret-value") {
				t.Fatalf("incorrect or unsafe diagnostic: %v", failure)
			}
		})
	}
}

func TestMalformedEntryLogsCauseWithoutContent(t *testing.T) {
	state := t.TempDir()
	t.Setenv("LAZYPASS_DEBUG", "1")
	t.Setenv("XDG_STATE_HOME", state)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "entry.gpg"), []byte("encrypted"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{fn: func(context.Context, []string, string, []string) ([]byte, error) {
		return []byte("secret-value\ninvalid metadata"), nil
	}}
	if _, err := New(root, runner).Read(context.Background(), vault.Path{"entry"}); !errors.Is(err, vault.ErrMalformed) {
		t.Fatalf("read error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(state, "lazypass", "debug.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `operation="pass decode" cause="invalid"`) || strings.Contains(string(data), "secret-value") {
		t.Fatalf("unsafe pass diagnostics: %q", data)
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
