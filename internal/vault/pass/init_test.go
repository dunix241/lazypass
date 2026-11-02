package pass

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lazypass/internal/vault"
)

func TestInitializeDelegatesToPass(t *testing.T) {
	r := &fakeRunner{fn: func(context.Context, []string, string, []string) ([]byte, error) { return nil, nil }}
	p := New(t.TempDir(), r)
	if err := p.Initialize(context.Background(), "ABCDEF123456"); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 1 || !reflect.DeepEqual(r.calls[0].args, []string{"init", "--", "ABCDEF123456"}) {
		t.Fatalf("calls=%v", r.calls)
	}
}

func TestInitializeRejectsExistingStoreAndInvalidRecipients(t *testing.T) {
	root := t.TempDir()
	p := New(root, nil)
	for _, recipient := range []string{"", " ", "--help", "key\nother"} {
		if err := p.Initialize(context.Background(), recipient); !errors.Is(err, vault.ErrInvalidPath) {
			t.Fatalf("accepted %q", recipient)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".gpg-id"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := p.Initialize(context.Background(), "ABCDEF"); !errors.Is(err, vault.ErrConflict) {
		t.Fatal("existing store reinitialized")
	}
}
