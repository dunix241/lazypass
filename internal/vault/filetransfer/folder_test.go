package filetransfer

import (
	"context"
	"errors"
	"fmt"
	"lazypass/internal/vault"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecursiveTransferAndInvalidDestinations(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "source", "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "source", "entry.age"), []byte("ciphertext fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	for _, target := range []string{"source/child"} {
		if _, err := Inspect(ctx, r, "source", target, ".age"); !errors.Is(err, vault.ErrInvalidPath) {
			t.Fatal("accepted recursive destination")
		}
	}
	tree, err := Inspect(ctx, r, "source", "copy", ".age")
	if err != nil {
		t.Fatal(err)
	}
	if err := ExecuteTree(ctx, r, "source", "copy", tree, false); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Stat("copy/empty"); err != nil {
		t.Fatal("empty folder lost")
	}
	if _, err := Inspect(ctx, r, "source", "copy", ".age"); !errors.Is(err, vault.ErrConflict) {
		t.Fatal("existing tree accepted")
	}
	tree, err = Inspect(ctx, r, "source", "moved", ".age")
	if err != nil {
		t.Fatal(err)
	}
	if err := ExecuteTree(ctx, r, "source", "moved", tree, true); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Stat("source"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("source remains")
	}
	data, err := os.ReadFile(filepath.Join(dir, "moved", "entry.age"))
	if err != nil || string(data) != "ciphertext fixture" {
		t.Fatal("ciphertext changed")
	}
	if err := os.WriteFile(filepath.Join(dir, "moved", "unknown"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(ctx, r, "moved", "other", ".age"); !errors.Is(err, vault.ErrUnsupported) {
		t.Fatal("unknown file silently accepted")
	}
}

func TestCancelActiveRecursiveTransfer(t *testing.T) {
	for _, move := range []bool{false, true} {
		t.Run(fmt.Sprintf("move=%t", move), func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "source"), 0700); err != nil {
				t.Fatal(err)
			}
			const count = 300
			for i := 0; i < count; i++ {
				if err := os.WriteFile(filepath.Join(dir, "source", fmt.Sprintf("%03d.age", i)), make([]byte, 4096), 0600); err != nil {
					t.Fatal(err)
				}
			}
			r, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			tree, err := Inspect(context.Background(), r, "source", "target", ".age")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- ExecuteTree(ctx, r, "source", "target", tree, move) }()
			deadline := time.Now().Add(5 * time.Second)
			for {
				if _, err := os.Stat(filepath.Join(dir, "target", tree.Files[0])); err == nil {
					break
				}
				select {
				case err := <-done:
					t.Fatalf("transfer finished before active cancellation: %v", err)
				default:
				}
				if time.Now().After(deadline) {
					t.Fatal("transfer did not start")
				}
				time.Sleep(100 * time.Microsecond)
			}
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("active cancel: %v", err)
			}
			if _, err := r.Stat("target"); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("partial destination remains")
			}
			entries, err := os.ReadDir(filepath.Join(dir, "source"))
			if err != nil || len(entries) != count {
				t.Fatal("active cancellation removed source files")
			}
		})
	}
}

func TestCanceledRecursiveMoveRetainsSourceAndRemovesPartialDestination(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "source"), 0700); err != nil {
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
	tree, err := Inspect(context.Background(), r, "source", "target", ".age")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ExecuteTree(ctx, r, "source", "target", tree, true); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := r.Stat("source/entry.age"); err != nil {
		t.Fatal("source lost on cancel")
	}
	if _, err := r.Stat("target"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial destination survived")
	}
}
