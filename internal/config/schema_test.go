package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSchemaCoversConfigFields(t *testing.T) {
	data, err := os.ReadFile("../../schemas/config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties  map[string]json.RawMessage `json:"properties"`
		Definitions map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		value      any
		properties map[string]json.RawMessage
	}{
		{Config{}, schema.Properties}, {Vault{}, schema.Definitions["vault"].Properties},
	} {
		typ := reflect.TypeOf(tc.value)
		for i := 0; i < typ.NumField(); i++ {
			name := strings.Split(typ.Field(i).Tag.Get("yaml"), ",")[0]
			if name != "-" && tc.properties[name] == nil {
				t.Errorf("schema missing %s.%s", typ.Name(), name)
			}
		}
	}
}

func TestSchemaURL(t *testing.T) {
	for _, version := range []string{"v1.2.3", "1.2.3"} {
		if got := schemaURL(version); got != "https://github.com/dunix241/lazypass/releases/download/v1.2.3/config.schema.json" {
			t.Fatalf("unexpected release schema: %s", got)
		}
	}
	if got := schemaURL("v1.2.3-dirty"); !strings.Contains(got, "/main/schemas/config.schema.json") {
		t.Fatalf("unexpected development schema: %s", got)
	}
}

func TestSavedConfigKeepsSchemaDirective(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := Defaults()
	for i := 0; i < 2; i++ {
		if err := cfg.Save(path); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(data), schemaDirective()) || strings.Count(string(data), "yaml-language-server:") != 1 {
			t.Fatalf("missing or duplicate schema directive: %s", data)
		}
		var saved Config
		if err := yaml.Unmarshal(data, &saved); err != nil {
			t.Fatal(err)
		}
		if saved.Updates != UpdatesNotify {
			t.Fatalf("config changed during save: %+v", saved)
		}
		cfg, err = Load(path)
		if err != nil {
			t.Fatal(err)
		}
	}
}
