package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"lazypass/internal/app"
	"lazypass/internal/config"
	"lazypass/internal/theme"

	"github.com/spf13/cobra"
)

func TestReadVaultPassword(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
		err   bool
	}{
		{"secret\n", "secret", false},
		{"secret\r\n", "secret", false},
		{"one\ntwo", "", true},
	} {
		got, err := readVaultPassword(bytes.NewBufferString(test.input))
		if (err != nil) != test.err || got != test.want {
			t.Fatalf("readVaultPassword(%q) = %q, %v", test.input, got, err)
		}
	}
	if _, err := readVaultPassword(bytes.NewBuffer(make([]byte, maxVaultPasswordBytes+1))); err == nil {
		t.Fatal("accepted oversized password input")
	}
}

func TestWriteResultsTextContainsOnlyPasswords(t *testing.T) {
	var output bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&output)
	results := []app.Result{{Password: "first"}, {Password: "second"}}
	if err := writeResults(command, results, "text"); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "first\nsecond\n" {
		t.Fatalf("text output = %q", got)
	}
}

func TestThemeCommands(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	command := &cobra.Command{}
	var output bytes.Buffer
	command.SetOut(&output)
	if err := runThemeList(command, nil); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != "catppuccin-mocha\ndracula\ngruvbox\nmidnight-rose\nmono\nnord\nonedark\nrose-pine\ntokyo-night\n" {
		t.Fatalf("list output = %q", got)
	}
	output.Reset()
	if err := runThemePath(command, nil); err != nil {
		t.Fatal(err)
	}
	dir, err := theme.Dir()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(output.String()); got != dir {
		t.Fatalf("path output = %q, want %q", got, dir)
	}
	output.Reset()
	if err := runThemeShow(command, []string{"nord"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "base: '#2E3440'") {
		t.Fatalf("show output = %q", output.String())
	}
}

func TestGenerateSavePreservesTheme(t *testing.T) {
	path := t.TempDir() + "/config.yaml"
	cfg := config.Defaults()
	cfg.Theme = "nord"
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	command := &cobra.Command{}
	command.SetOut(&bytes.Buffer{})
	addOptions(command)
	command.PersistentFlags().String("config", "", "")
	command.Flags().Int("count", 1, "")
	command.Flags().String("format", "text", "")
	command.Flags().Bool("no-save", false, "")
	if err := command.ParseFlags([]string{"--config", path, "--length", "24"}); err != nil {
		t.Fatal(err)
	}
	if err := runGenerate(command, nil); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Theme != "nord" {
		t.Fatalf("saved theme = %q, want nord", loaded.Theme)
	}
}

func TestWriteResultsJSONUsesNDJSON(t *testing.T) {
	var output bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&output)
	results := []app.Result{{Password: "first", Length: 20, EntropyBits: 100, Strength: "Strong"}, {Password: "second", Length: 20, EntropyBits: 100, Strength: "Strong"}}
	if err := writeResults(command, results, "json"); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d JSON lines", len(lines))
	}
	for _, line := range lines {
		var result app.Result
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			t.Fatalf("invalid JSON %q: %v", line, err)
		}
		if result.Password == "" || result.Strength != "Strong" {
			t.Fatalf("unexpected JSON result: %+v", result)
		}
	}
}

func TestWriteResultsRejectsUnknownFormat(t *testing.T) {
	command := &cobra.Command{}
	if err := writeResults(command, []app.Result{{Password: "password"}}, "yaml"); err == nil {
		t.Fatal("expected invalid format error")
	}
}

func TestExitCode(t *testing.T) {
	if got := ExitCode(usage(assertionError("invalid flag"))); got != 2 {
		t.Fatalf("usage exit code = %d, want 2", got)
	}
	if got := ExitCode(assertionError("clipboard unavailable")); got != 1 {
		t.Fatalf("runtime exit code = %d, want 1", got)
	}
}

func TestVaultCommandStartsInteractiveVaultRoute(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"vault"})
	if err != nil {
		t.Fatal(err)
	}
	if command == nil || command.RunE == nil {
		t.Fatal("vault command must have an interactive handler")
	}
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
