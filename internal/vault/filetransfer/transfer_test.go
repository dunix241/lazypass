package filetransfer

import (
	"context"
	"errors"
	"lazypass/internal/vault"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeTransferPreservesBytesAndNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	data := []byte("encrypted fixture\x00\xff\r\n")
	if err := os.WriteFile(filepath.Join(dir, "source"), data, 0600); err != nil {
		t.Fatal(err)
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	if err := Execute(ctx, r, "source", "copy", false); err != nil {
		t.Fatal(err)
	}
	if err := Execute(ctx, r, "source", "copy", true); !errors.Is(err, vault.ErrConflict) {
		t.Fatalf("overwrite: %v", err)
	}
	if err := Execute(ctx, r, "source", "moved", true); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Stat("source"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("source survived move")
	}
	for _, name := range []string{"copy", "moved"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != string(data) {
			t.Fatal("native bytes changed")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := Execute(canceled, r, "copy", "canceled", true); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := r.Stat("canceled"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canceled move published")
	}
}

func TestRejectSymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("source", filepath.Join(dir, "link")); err != nil {
		t.Skip(err)
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := Execute(context.Background(), r, "link", "copy", false); !errors.Is(err, vault.ErrInvalidPath) {
		t.Fatal(err)
	}
}
