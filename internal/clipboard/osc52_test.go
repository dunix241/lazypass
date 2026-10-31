package clipboard

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOSC52PreservesBytesAndFramesClipboardSelection(t *testing.T) {
	for _, text := range []string{"public-marker", "世界\n\x00\x1b", strings.Repeat("x", maxTerminalBytes)} {
		var output bytes.Buffer
		if err := writeOSC52(&output, text); err != nil {
			t.Fatal(err)
		}
		sequence := output.String()
		if !strings.HasPrefix(sequence, "\x1b]52;c;") || !strings.HasSuffix(sequence, "\a") {
			t.Fatal("invalid OSC 52 framing")
		}
		decoded, err := base64.StdEncoding.DecodeString(sequence[7 : len(sequence)-1])
		if err != nil || string(decoded) != text {
			t.Fatal("clipboard bytes changed")
		}
	}
}

type failingWriter struct{ short bool }

func (w failingWriter) Write(p []byte) (int, error) {
	if w.short {
		return len(p) - 1, nil
	}
	return 0, errors.New("secret-value in external error")
}

func TestOSC52WriteFailuresAreSafe(t *testing.T) {
	for _, writer := range []io.Writer{failingWriter{}, failingWriter{short: true}} {
		if err := writeOSC52(writer, "secret-value"); err != errTerminalWrite {
			t.Fatal("unsafe write error")
		}
	}
	var output bytes.Buffer
	if err := writeOSC52(&output, strings.Repeat("x", maxTerminalBytes+1)); err != errTerminalLimit || output.Len() != 0 {
		t.Fatal("oversized payload dispatched")
	}
}

func TestTmuxPassthroughEscapesOSC52(t *testing.T) {
	var output bytes.Buffer
	if err := writeTerminalSequence(&output, "marker", true); err != nil {
		t.Fatal(err)
	}
	if output.String() != "\x1bPtmux;\x1b\x1b]52;c;bWFya2Vy\a\x1b\\" {
		t.Fatal("invalid tmux passthrough framing")
	}
}

func TestRemoteCopySkipsHostBackends(t *testing.T) {
	for _, failure := range []bool{false, true} {
		calls := 0
		err := copyWith("marker", dependencies{
			remote: true, wayland: true,
			native:   func(string) error { t.Fatal("remote native backend called"); return nil },
			lookPath: func(string) (string, error) { t.Fatal("remote host utility called"); return "", nil },
			terminal: func(text string) error {
				calls++
				if text != "marker" {
					t.Fatal("payload changed")
				}
				if failure {
					return errTerminalUnavailable
				}
				return nil
			},
		})
		if calls != 1 || (err != nil) != failure {
			t.Fatal("remote routing failed")
		}
	}
}

func TestLocalCopyFallsBackToTerminal(t *testing.T) {
	calls := 0
	err := copyWith("marker", dependencies{
		native:   func(string) error { return errors.New("unavailable") },
		lookPath: func(string) (string, error) { return "", os.ErrNotExist },
		terminal: func(string) error { calls++; return nil },
	})
	if err != nil || calls != 1 {
		t.Fatal("terminal fallback not used")
	}
	if err := copyWith("marker", dependencies{native: func(string) error { return nil }, terminal: func(string) error { t.Fatal("terminal called after host success"); return nil }}); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDetection(t *testing.T) {
	for _, key := range []string{"SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY", "MOSH_CONNECTION"} {
		if !remoteSession(func(name string) string {
			if name == key {
				return "set"
			}
			return ""
		}) {
			t.Fatalf("missed %s", key)
		}
	}
	if remoteSession(func(string) string { return "" }) {
		t.Fatal("local session detected as remote")
	}
}

func TestTerminalFailureDiagnosticsContainNoPayload(t *testing.T) {
	state := t.TempDir()
	t.Setenv("LAZYPASS_DEBUG", "1")
	t.Setenv("XDG_STATE_HOME", state)
	err := copyWith("secret-value", dependencies{remote: true, terminal: func(string) error { return errTerminalWrite }})
	if err != errTerminalWrite {
		t.Fatal("terminal failure not returned")
	}
	data, err := os.ReadFile(filepath.Join(state, "lazypass", "debug.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "clipboard osc52") || strings.Contains(string(data), "secret-value") || strings.Contains(string(data), base64.StdEncoding.EncodeToString([]byte("secret-value"))) {
		t.Fatal("unsafe terminal diagnostics")
	}
}
