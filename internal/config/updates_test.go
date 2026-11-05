package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestUpdateModes(t *testing.T) {
	for _, tc := range []struct {
		input   string
		want    UpdateMode
		invalid bool
	}{
		{"{}", UpdatesNotify, false},
		{"updates: off", UpdatesOff, false},
		{"updates: notify", UpdatesNotify, false},
		{"updates: auto", UpdatesAuto, false},
		{"updates: {check: false}", UpdatesOff, false},
		{"updates: {check: true}", UpdatesNotify, false},
		{"updates: true", "", true},
		{"updates: automatic", "", true},
		{"updates: {mode: auto, cacheMaxAge: 15m}", UpdatesAuto, false},
		{"updates: {mode: notify, cacheMaxAge: 0s}", "", true},
		{"updates: {mode: notify, cacheMaxAge: -1h}", "", true},
		{"updates: {mode: notify, cacheMaxAge: tomorrow}", "", true},
		{"updates: {mode: auto, cacheMaxAge: 1h30m}", UpdatesAuto, false},
		{"updates: {mode: auto, cacheMaxAge: 9223372036854775807h}", "", true},
		{"updates: {mode: auto, cacheMaxAg: 1h}", "", true},
		{"updates: {mode: [auto]}", "", true},
		{"updates: {mode: auto, mode: off}", "", true},
		{"updates: {check: false, check: true}", "", true},
		{"updates: {check: false, cacheMaxAge: 5m}", "", true},
	} {
		t.Run(tc.input, func(t *testing.T) {
			cfg := Defaults()
			doc, _, err := migrateConfig([]byte(tc.input))
			if err == nil {
				err = doc.Decode(&cfg)
			}
			if (err != nil) != tc.invalid {
				t.Fatalf("unexpected decode: %v", err)
			}
			if tc.invalid {
				return
			}
			if cfg.Updates.Mode != tc.want {
				t.Fatalf("mode %q, want %q", cfg.Updates.Mode, tc.want)
			}
			if cfg.Updates.CacheMaxAge <= 0 {
				t.Fatal("missing cache age")
			}
			data, err := yaml.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			var saved struct {
				Updates yaml.Node `yaml:"updates"`
			}
			if err := yaml.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if saved.Updates.Kind != yaml.MappingNode {
				t.Fatalf("updates not saved as mapping: %s", data)
			}
		})
	}
}

func TestUpdateMigrationPreservesComments(t *testing.T) {
	document, changed, err := migrateConfig([]byte("updates:\n  # user preference\n  check: false # keep disabled\n"))
	if err != nil || !changed {
		t.Fatalf("migration: changed=%t err=%v", changed, err)
	}
	data, err := encodeYAML(document)
	if err != nil {
		t.Fatal(err)
	}
	for _, comment := range []string{"# user preference", "# keep disabled"} {
		if !strings.Contains(string(data), comment) {
			t.Fatalf("lost comment %q: %s", comment, data)
		}
	}
}

func TestUpdateCacheDefault(t *testing.T) {
	if Defaults().Updates.CacheMaxAge != time.Hour {
		t.Fatal("default cache age must be one hour")
	}
}

func TestUpdateSettingsMigrationIsPersisted(t *testing.T) {
	for _, input := range []string{"updates: auto\n", "updates: {check: false}\n"} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte("# preferences\n"+input), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "cacheMaxAge: 1h0m0s") || !strings.Contains(string(data), "# preferences") {
			t.Fatalf("migration not persisted: %s", data)
		}
		again, err := Load(path)
		if err != nil || again.Updates != cfg.Updates {
			t.Fatalf("migration changed settings: %+v %v", again.Updates, err)
		}
	}
}
