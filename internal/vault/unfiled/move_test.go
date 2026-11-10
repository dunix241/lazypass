package unfiled

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"lazypass/internal/vault"
)

func TestPlanPreservesSubdirectoriesAndRejectsCollisions(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "source", "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "source", "entry.age"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	moves, err := Plan(context.Background(), r, vault.Path{"source"}, ".age")
	if err != nil || len(moves) != 2 {
		t.Fatalf("moves: %v %v", moves, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, Directory, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Plan(context.Background(), r, vault.Path{"source"}, ".age"); !errors.Is(err, vault.ErrConflict) {
		t.Fatal("collision accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "source", "entry.age")); err != nil {
		t.Fatal("preflight modified source")
	}
}

func TestPlanCancellationAndSymlinkDoNotModifySource(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "source"), 0700); err != nil {
		t.Fatal(err)
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Plan(ctx, r, vault.Path{"source"}, ".age"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(dir, "source", "nested")); err != nil {
		t.Fatal(err)
	}
	if _, err := Plan(context.Background(), r, vault.Path{"source"}, ".age"); !errors.Is(err, vault.ErrInvalidPath) {
		t.Fatalf("symlink accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, Directory)); !os.IsNotExist(err) {
		t.Fatal("preflight created destination")
	}
}

func TestPlanRejectsEntryDirectoryNameCollision(t *testing.T) {
	for _, sourceFolder := range []bool{false, true} {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "source"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(dir, Directory), 0700); err != nil {
			t.Fatal(err)
		}
		if sourceFolder {
			if err := os.Mkdir(filepath.Join(dir, "source", "same"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, Directory, "same.gpg"), []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(filepath.Join(dir, "source", "same.gpg"), []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(dir, Directory, "same"), 0700); err != nil {
				t.Fatal(err)
			}
		}
		r, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Plan(context.Background(), r, vault.Path{"source"}, ".gpg")
		r.Close()
		if !errors.Is(err, vault.ErrConflict) {
			t.Fatalf("logical collision accepted: %v", err)
		}
	}
}
