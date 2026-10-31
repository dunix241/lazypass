package clipboard

import (
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
)

const maxTerminalBytes = 64 * 1024

var (
	errTerminalUnavailable = errors.New("terminal clipboard unavailable: an interactive terminal is required")
	errTerminalLimit       = errors.New("terminal clipboard payload exceeds 64 KiB")
	errTerminalWrite       = errors.New("terminal clipboard write failed")
	terminalMu             sync.Mutex
)

// writeOSC52 confirms dispatch, not receipt: OSC 52 has no write acknowledgement.
// Keep bytes intact, including NUL; Base64 safely transports control characters.
func writeOSC52(w io.Writer, text string) error {
	return writeTerminalSequence(w, text, false)
}

func writeTerminalSequence(w io.Writer, text string, tmux bool) error {
	if len(text) > maxTerminalBytes {
		return errTerminalLimit
	}
	sequence := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte(text)) + "\a"
	if tmux {
		sequence = "\x1bPtmux;" + strings.ReplaceAll(sequence, "\x1b", "\x1b\x1b") + "\x1b\\"
	}
	n, err := io.WriteString(w, sequence)
	if err != nil || n != len(sequence) {
		return errTerminalWrite
	}
	return nil
}

func copyTerminal(text string) error {
	if len(text) > maxTerminalBytes {
		return errTerminalLimit
	}
	terminalMu.Lock()
	defer terminalMu.Unlock()
	tty, err := openTerminal()
	if err != nil {
		return errTerminalUnavailable
	}
	defer tty.Close()
	return writeTerminalSequence(tty, text, os.Getenv("TMUX") != "")
}
