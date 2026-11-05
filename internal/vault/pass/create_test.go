package pass

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"lazypass/internal/vault"
)

func TestCreatePublishesWithoutReplacingConcurrentEntry(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gpg-id"), []byte("fixture-key"), 0600); err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	r := &createRunner{run: func(env []string) error {
		stage := ""
		for _, item := range env {
			if strings.HasPrefix(item, "PASSWORD_STORE_DIR=") {
				stage = strings.TrimPrefix(item, "PASSWORD_STORE_DIR=")
			}
		}
		if !slices.Contains(env, "GIT_DIR="+filepath.Join(stage, ".git")) {
			return errors.New("staging did not isolate Git")
		}
		ready <- struct{}{}
		<-release
		return os.WriteFile(filepath.Join(stage, "entry.gpg"), []byte("ciphertext"), 0600)
	}}
	p := New(root, r)
	results := make(chan error, 2)
	for range 2 {
		go func() {
			results <- p.Create(context.Background(), vault.Entry{Path: vault.Path{"hello"}, Password: "fixture"})
		}()
	}
	<-ready
	<-ready
	close(release)
	a, b := <-results, <-results
	if (a != nil || !errors.Is(b, vault.ErrConflict)) && (b != nil || !errors.Is(a, vault.ErrConflict)) {
		t.Fatalf("expected one creation and one conflict: %v %v", a, b)
	}
	nodes, err := p.List(context.Background(), nil)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("unexpected metadata: %v %v", nodes, err)
	}
}

func TestCreateRejectsSignatureSymlinkAndCleansStaging(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gpg-id"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".gpg-id"), filepath.Join(root, ".gpg-id.sig")); err != nil {
		t.Skip(err)
	}
	p := New(root, &createRunner{run: func([]string) error { t.Fatal("encryption started with invalid signature"); return nil }})
	if err := p.Create(context.Background(), vault.Entry{Path: vault.Path{"entry"}, Password: "fixture"}); !errors.Is(err, vault.ErrInvalidPath) {
		t.Fatalf("got %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".lazypass-create-") {
			t.Fatal("staging directory leaked")
		}
	}
}

type createRunner struct{ run func([]string) error }

func (r *createRunner) Run(_ context.Context, _ string, _ []string, _ io.Reader, env []string) ([]byte, error) {
	return nil, r.run(env)
}
