package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"lazypass/internal/app"

	"github.com/spf13/cobra"
)

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

type assertionError string

func (e assertionError) Error() string { return string(e) }
