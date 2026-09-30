package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisabledByDefault(t *testing.T) {
	t.Setenv("LAZYPASS_DEBUG", "")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if Enabled() {
		t.Fatal("debug logging should be off without LAZYPASS_DEBUG")
	}
	Failure("config load", IO)
	entries, err := os.ReadDir(filepath.Join(os.Getenv("XDG_STATE_HOME"), "lazypass"))
	if err == nil && len(entries) > 0 {
		t.Fatal("disabled debug logging created files")
	}
}

func TestEnabledWritesLog(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv("LAZYPASS_DEBUG", "1")
	if !Enabled() {
		t.Fatal("LAZYPASS_DEBUG=1 should enable debug logging")
	}
	Failure("vault copy", Locked)
	data, err := os.ReadFile(filepath.Join(state, "lazypass", "debug.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `operation="vault copy"`) || !strings.Contains(string(data), `cause="locked"`) {
		t.Fatalf("log line missing operation detail: %q", data)
	}
	info, err := os.Stat(Path())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("debug log must be private: info=%v err=%v", info, err)
	}
}
