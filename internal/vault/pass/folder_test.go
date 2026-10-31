package pass

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazypass/internal/vault"
)

func TestDeletingLastChildPreservesContainingFolder(t *testing.T) {
	for _, folder := range []bool{false, true} {
		root := t.TempDir()
		parent := filepath.Join(root, "hello")
		if err := os.Mkdir(parent, 0700); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(parent, "world.gpg")
		if folder {
			target = filepath.Join(parent, "world")
			if err := os.Mkdir(target, 0700); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(target, []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		r := &fakeRunner{fn: func(_ context.Context, _ []string, _ string, _ []string) ([]byte, error) {
			if err := os.RemoveAll(target); err != nil {
				t.Fatal(err)
			}
			// Reproduce pass's rmdir -p: stop at the first nonempty ancestor.
			for dir := parent; strings.HasPrefix(dir, root); dir = filepath.Dir(dir) {
				if err := os.Remove(dir); err != nil {
					break
				}
			}
			return nil, nil
		}}
		p := New(root, r)
		path := vault.Path{"hello", "world"}
		var err error
		if folder {
			err = p.DeleteFolder(context.Background(), path)
		} else {
			err = p.Delete(context.Background(), path)
		}
		if err != nil {
			t.Fatal(err)
		}
		nodes, err := p.List(context.Background(), vault.Path{"hello"})
		if err != nil || len(nodes) != 0 {
			t.Fatalf("containing folder lost or not empty: %v %v", nodes, err)
		}
		entries, err := os.ReadDir(parent)
		if err != nil || len(entries) != 0 {
			t.Fatal("temporary keep file not cleaned up")
		}
	}
}

func TestCreateFolderNestedAndListed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gpg-id"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	p := New(root, nil)
	if err := p.CreateFolder(context.Background(), vault.Path{"team", "mail"}); err != nil {
		t.Fatal(err)
	}
	nodes, err := p.List(context.Background(), vault.Path{"team"})
	if err != nil || len(nodes) != 1 || nodes[0].Name != "mail" || nodes[0].Kind != vault.FolderNode {
		t.Fatalf("nodes=%v err=%v", nodes, err)
	}
	info, err := os.Stat(filepath.Join(root, "team", "mail"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatal("folder mode not private")
	}
	if err := p.CreateFolder(context.Background(), vault.Path{"team", "mail"}); err != nil {
		t.Fatal("existing folder should be reusable")
	}
}

func TestDeleteFolderUsesRecursivePassAndRejectsRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "folder", "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{fn: func(context.Context, []string, string, []string) ([]byte, error) { return nil, nil }}
	p := New(root, r)
	if err := p.DeleteFolder(context.Background(), nil); !errors.Is(err, vault.ErrInvalidPath) {
		t.Fatal("root deletion allowed")
	}
	if err := p.DeleteFolder(context.Background(), vault.Path{"folder"}); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 1 {
		t.Fatal("missing pass invocation")
	}
	args := r.calls[0].args
	want := []string{"rm", "--recursive", "--force", "--", "folder/"}
	if len(args) != len(want) {
		t.Fatalf("args=%v", args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args=%v", args)
		}
	}
}

func TestCreateFolderRejectsInvalidSymlinkAndEntry(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gpg-id"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "entry.gpg"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	p := New(root, nil)
	if err := p.CreateFolder(context.Background(), vault.Path{"entry", "child"}); err != nil {
		t.Fatalf("existing value name blocked folder creation: %v", err)
	}
	for _, test := range []struct {
		path vault.Path
		want error
	}{{nil, vault.ErrInvalidPath}, {vault.Path{".."}, vault.ErrInvalidPath}, {vault.Path{"link", "child"}, vault.ErrInvalidPath}} {
		if err := p.CreateFolder(context.Background(), test.path); !errors.Is(err, test.want) {
			t.Fatalf("got %v want %v", err, test.want)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "child")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("created outside store")
	}
	uninitialized := New(t.TempDir(), nil)
	if err := uninitialized.CreateFolder(context.Background(), vault.Path{"folder"}); !errors.Is(err, vault.ErrUninitialized) {
		t.Fatal("uninitialized folder creation allowed")
	}
}
