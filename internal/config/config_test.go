package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazypass/internal/generator"
	"lazypass/internal/theme"
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

func TestWithOptionsPreservesTheme(t *testing.T) {
	c := Defaults()
	c.Theme = "nord"
	opts := generator.Defaults()
	opts.Length = 32
	updated := c.WithOptions(opts)
	if updated.Theme != "nord" {
		t.Fatalf("theme = %q, want nord", updated.Theme)
	}
}

func TestVaultConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	c := Defaults()
	c.Vault = Vault{Provider: "pass", StoreDir: "/vault"}
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Vault != c.Vault {
		t.Fatalf("vault = %#v, want %#v", loaded.Vault, c.Vault)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "vault:\n  provider: pass\n  storeDir: /vault\n") {
		t.Fatalf("config does not use two-space indentation:\n%s", data)
	}
}

func TestLoadLegacyConfigDefaultsTheme(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("length: 20\nupper: true\nlower: true\nnumbers: true\nsymbols: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Theme != theme.DefaultName() {
		t.Fatalf("theme = %q, want %q", loaded.Theme, theme.DefaultName())
	}
	if !loaded.NerdFont {
		t.Fatal("legacy config should default Nerd Font support on")
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

func TestCorruptFileIsPreservedAndReported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("::: not yaml :::"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected corrupt config error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "::: not yaml :::" {
		t.Fatalf("config content changed: %q", data)
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

func TestInvalidValuesArePreservedAndReported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	invalid := "length: 999\nupper: false\nlower: false\nnumbers: false\nsymbols: false\n"
	if err := os.WriteFile(path, []byte(invalid), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected invalid config error")
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != invalid {
		t.Fatalf("config content changed: %q", stored)
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
