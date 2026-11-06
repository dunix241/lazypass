package bitwarden

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"lazypass/internal/vault"
)

func TestOrganizationReadOnlyLive(t *testing.T) {
	dataDir := os.Getenv("LAZYPASS_BW_READONLY_DIR")
	if dataDir == "" {
		t.Skip("set isolated CLI directory and password file for read-only verification")
	}
	p := New(dataDir, os.Getenv("LAZYPASS_BW_READONLY_SERVER"), func() (string, error) {
		data, err := os.ReadFile(os.Getenv("LAZYPASS_BW_READONLY_PASSWORD_FILE"))
		defer clear(data)
		return strings.TrimRight(string(data), "\r\n"), err
	}, nil)
	defer p.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root, err := p.List(ctx, nil)
	if err != nil {
		t.Fatal("could not list root")
	}
	if len(root) != 2 {
		t.Fatal("expected virtual roots")
	}
	organizations, err := p.List(ctx, vault.Path{"Organizations"})
	if err != nil {
		t.Fatal("could not list organizations")
	}
	collections, entries := 0, 0
	for _, org := range organizations {
		groups, err := p.List(ctx, org.Path)
		if err != nil {
			t.Fatal("could not list collections")
		}
		collections += len(groups)
		for _, group := range groups {
			nodes, err := p.List(ctx, group.Path)
			if err != nil {
				t.Fatal("could not list collection entries")
			}
			entries += len(nodes)
		}
	}
	t.Logf("read-only browser counts: organizations=%d collections=%d entry appearances=%d", len(organizations), collections, entries)
	if len(organizations) == 0 || entries == 0 {
		t.Fatal("organization browser unexpectedly empty")
	}
}
