package config

import (
	"testing"

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
	} {
		t.Run(tc.input, func(t *testing.T) {
			cfg := Defaults()
			err := yaml.Unmarshal([]byte(tc.input), &cfg)
			if (err != nil) != tc.invalid {
				t.Fatalf("unexpected decode: %v", err)
			}
			if tc.invalid {
				return
			}
			if cfg.Updates != tc.want {
				t.Fatalf("mode %q, want %q", cfg.Updates, tc.want)
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
			if saved.Updates.Kind != yaml.ScalarNode || saved.Updates.Value != string(tc.want) {
				t.Fatalf("mode not saved as scalar: %s", data)
			}
		})
	}
}
