package state

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestVaultStateIsScopedByConfig(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	for _, item := range []struct{ config, name string }{{"first.yaml", "personal"}, {"second.yaml", "work"}} {
		if err := SaveVault(item.config, item.name); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []struct{ config, name string }{{"first.yaml", "personal"}, {"second.yaml", "work"}} {
		got, err := LastVault(item.config)
		if err != nil || got != item.name {
			t.Fatalf("state=%q err=%v", got, err)
		}
	}
	file, err := path()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("state is not private")
	}
	if filepath.Base(file) != "state.yaml" {
		t.Fatal("unexpected state path")
	}
}

func TestResolveVaultMigratesOnceAndPrefersState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	config := filepath.Join(t.TempDir(), "config.yaml")
	got, err := ResolveVault(config, "legacy")
	if err != nil || got != "legacy" {
		t.Fatalf("migration: %q %v", got, err)
	}
	got, err = ResolveVault(config, "stale")
	if err != nil || got != "legacy" {
		t.Fatalf("state precedence: %q %v", got, err)
	}
	if err := SaveVault(config, "work"); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveVault(config, "legacy")
	if err != nil || got != "work" {
		t.Fatal("selection replaced by legacy config")
	}
}

func TestInvalidStateIsPreserved(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	file, err := path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	invalid := []byte("lastVaults: [broken\n")
	if err := os.WriteFile(file, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	if err := SaveVault("config.yaml", "work"); err == nil {
		t.Fatal("invalid state overwritten")
	}
	data, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(data, invalid) {
		t.Fatal("invalid state changed")
	}
}

func TestEmptyConfigDoesNotWriteState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := SaveVault("", "work"); err != nil {
		t.Fatal(err)
	}
	file, err := path()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("empty config created state file")
	}
}
