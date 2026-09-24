package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	c := Defaults()
	if err := c.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded != c {
		t.Fatalf("round-trip mismatch: %+v vs %+v", loaded, c)
	}
}

func TestMissingFileGivesDefaults(t *testing.T) {
	loaded, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded != Defaults() {
		t.Fatalf("expected defaults, got %+v", loaded)
	}
}

func TestCorruptFileBacksUpAndDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("::: not yaml :::"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded != Defaults() {
		t.Fatalf("expected defaults on corrupt, got %+v", loaded)
	}
	if _, err := os.Stat(path + ".corrupt.bak"); err != nil {
		t.Fatalf("expected backup file: %v", err)
	}
}

func TestEnvOverride(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "custom.yaml")
	t.Setenv("LAZYPASS_CONFIG", custom)
	got, err := ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != custom {
		t.Fatalf("got %q want %q", got, custom)
	}
}

func TestExplicitPathOverridesEnvironment(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "explicit.yaml")
	t.Setenv("LAZYPASS_CONFIG", filepath.Join(dir, "environment.yaml"))
	got, err := ResolvePath(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if got != explicit {
		t.Fatalf("got %q want %q", got, explicit)
	}
}

func TestInvalidValuesBackUpAndRestoreDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	invalid := "length: 999\nupper: false\nlower: false\nnumbers: false\nsymbols: false\n"
	if err := os.WriteFile(path, []byte(invalid), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded != Defaults() {
		t.Fatalf("expected defaults for invalid config, got %+v", loaded)
	}
	backup, err := os.ReadFile(path + ".corrupt.bak")
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backup) != invalid {
		t.Fatalf("backup content changed: %q", backup)
	}
}

func TestSaveCreatesPrivateFileAndParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	if err := Defaults().Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config permissions = %o, want 600", got)
	}
}
