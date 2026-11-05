package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultMigrationPersistsOnLoad(t *testing.T) {
	for _, body := range []string{
		"vault:\n  provider: pass # backend\n  storeDir: /fixture\n",
		"vault:\n  - name: work\n    provider: pass # backend\n    storeDir: /fixture\n",
		"vaults:\n  provider: pass # backend\n  storeDir: /fixture\n",
	} {
		t.Run(body, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			input := "# user comment\nlength: 32 # password length\n" + body
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Vaults) != 1 || cfg.Length != 32 {
				t.Fatalf("migration changed configuration: %+v", cfg)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"# user comment", "# password length", "# backend", "vaults:\n  -"} {
				if !strings.Contains(string(data), want) {
					t.Fatalf("migration lost %q: %s", want, data)
				}
			}
			before, _ := os.Stat(path)
			if _, err := Load(path); err != nil {
				t.Fatal(err)
			}
			after, _ := os.Stat(path)
			if !os.SameFile(before, after) {
				t.Fatal("migration rewrote canonical configuration")
			}
		})
	}
}

func TestInvalidLegacyConfigIsNotMigrated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	input := []byte("length: 1\nvault:\n  provider: pass\n")
	if err := os.WriteFile(path, input, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("invalid config accepted")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(input, got) {
		t.Fatal("invalid config modified")
	}
}

func TestVaultMigrationRemovesShadowedLegacyKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("vault:\n  provider: pass\nvaults: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Vaults) != 0 {
		t.Fatal("legacy key overrode canonical list")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "\nvault:") {
		t.Fatal("shadowed legacy key retained")
	}
}
