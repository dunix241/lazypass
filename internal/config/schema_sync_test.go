package config

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazypass/internal/build"
)

func TestSyncSchemaPreservesConfig(t *testing.T) {
	for _, body := range []string{
		"# My settings\nlength: 32 # keep this\nupdates: auto\n",
		"# My settings\r\nlength: 32 # keep this\r\nupdates: auto\r\n",
		"length: 32",
		"symbolSet: |\n  # yaml-language-server: $schema=literal-symbols\n",
	} {
		t.Run(body, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			original := "# yaml-language-server: $schema=https://example.org/old.json\n" + body
			if err := os.WriteFile(path, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			if err := SyncSchema(context.Background(), path); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(string(got), body) || !bytes.Contains(got, []byte(schemaURL(build.Version))) {
				t.Fatalf("config not preserved: %q", got)
			}
			before, _ := os.Stat(path)
			if err := SyncSchema(context.Background(), path); err != nil {
				t.Fatal(err)
			}
			after, _ := os.Stat(path)
			if !os.SameFile(before, after) {
				t.Fatal("unchanged directive rewrote file")
			}
		})
	}
}

func TestSyncSchemaMissingAndCancelled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := SyncSchema(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("sync created missing config")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := SyncSchema(ctx, path); err != context.Canceled {
		t.Fatalf("cancellation ignored: %v", err)
	}
}

func TestSchemaDirectiveInsertionAndDeduplication(t *testing.T) {
	body := "# keep comments\nupdates: notify\n"
	for _, input := range []string{
		body,
		"# yaml-language-server: $schema=old\n# yaml-language-server: $schema=older\n" + body,
	} {
		got := withSchemaDirective([]byte(input))
		if !strings.HasSuffix(string(got), body) || bytes.Count(got, []byte(schemaComment)) != 1 {
			t.Fatalf("unexpected directive update: %q", got)
		}
	}
}
