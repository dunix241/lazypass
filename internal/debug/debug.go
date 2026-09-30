// Package debug records opt-in, secret-free failure diagnostics.
package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Operation string

type Cause uint8

const (
	Unknown Cause = iota
	IO
	Invalid
	Unavailable
	NotFound
	Locked
	Agent
	Pinentry
	Terminal
	MissingKey
	Passphrase
	Decrypt
	Conflict
	Timeout
	Canceled
	Clipboard
)

var causeNames = [...]string{
	Unknown:     "unknown",
	IO:          "io",
	Invalid:     "invalid",
	Unavailable: "unavailable",
	NotFound:    "not_found",
	Locked:      "locked",
	Agent:       "agent",
	Pinentry:    "pinentry",
	Terminal:    "terminal",
	MissingKey:  "missing_key",
	Passphrase:  "passphrase",
	Decrypt:     "decrypt",
	Conflict:    "conflict",
	Timeout:     "timeout",
	Canceled:    "canceled",
	Clipboard:   "clipboard",
}

func (c Cause) String() string {
	if int(c) < len(causeNames) {
		return causeNames[c]
	}
	return causeNames[Unknown]
}

func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LAZYPASS_DEBUG"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func Path() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "lazypass", "debug.log")
}

type ProcessState struct {
	ExitCode int
	StdinTTY bool
	GPGTTY   bool
	SSH      bool
	Display  bool
}

// Failure accepts only static operation names and predefined causes, never
// secrets, paths, or untrusted error strings.
func Failure(operation Operation, cause Cause) {
	if !Enabled() {
		return
	}
	write(fmt.Sprintf("operation=%q cause=%q", operation, cause.String()))
}

func ProcessFailure(operation Operation, cause Cause, state ProcessState) {
	if !Enabled() {
		return
	}
	write(fmt.Sprintf("operation=%q cause=%q exit=%d stdin_tty=%t gpg_tty=%t ssh=%t display=%t",
		operation, cause.String(), state.ExitCode, state.StdinTTY, state.GPGTTY, state.SSH, state.Display))
}

func write(event string) {
	path := Path()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	if err := f.Chmod(0o600); err != nil {
		return
	}
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), event)
}
