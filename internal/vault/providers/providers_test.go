package providers

import (
	"path/filepath"
	"slices"
	"testing"

	"lazypass/internal/config"
	"lazypass/internal/vault"
)

func TestNamedAgeDefaultsAreIndependent(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a, ai, err := AgePaths(config.Vault{Name: "personal"})
	if err != nil {
		t.Fatal(err)
	}
	b, bi, err := AgePaths(config.Vault{Name: "work"})
	if err != nil {
		t.Fatal(err)
	}
	if a == b || ai == bi || filepath.Base(a) != "vault-personal" {
		t.Fatal("named vault defaults collide")
	}
	custom := config.Vault{StoreDir: "/custom/store", IdentityFile: "/custom/key"}
	store, key, err := AgePaths(custom)
	if err != nil || store != custom.StoreDir || key != custom.IdentityFile {
		t.Fatal("explicit paths changed")
	}
}

func TestBuiltinsSupportAllEntryTypes(t *testing.T) {
	for _, name := range []string{"age", "pass", "bitwarden"} {
		provider, err := Builtins().Open(config.Vault{Name: name, Provider: name, StoreDir: t.TempDir(), IdentityFile: "fixture"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		types, ok := provider.(vault.EntryTypes)
		if !ok || !slices.Equal(types.SupportedEntryTypes(), vault.AllEntryTypes()) {
			t.Fatalf("%s does not support all entry types", name)
		}
	}
}

func TestRegistrySupportsAdditionalFactory(t *testing.T) {
	registry := Builtins()
	registry["future"] = registry["pass"]
	p, err := registry.Open(config.Vault{Provider: "future", StoreDir: t.TempDir()}, nil)
	if err != nil || p.Name() != "pass" {
		t.Fatal("custom factory not used")
	}
	if _, err := registry.Open(config.Vault{Provider: "unknown"}, nil); err == nil {
		t.Fatal("unknown provider silently accepted")
	}
}
