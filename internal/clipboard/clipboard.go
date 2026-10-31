// Package clipboard copies text to the system clipboard without exposing it to
// stdout. SSH sessions use OSC 52; local sessions prefer the host clipboard.
package clipboard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"lazypass/internal/debug"

	"github.com/atotto/clipboard"
)

type dependencies struct {
	remote   bool
	terminal func(string) error
	wayland  bool
	native   func(string) error
	lookPath func(string) (string, error)
	run      func(string, []string, string) error
}

// Copy writes text to the client's terminal clipboard remotely, or the best
// available host/terminal destination locally.
func Copy(text string) error {
	return copyWith(text, dependencies{
		wayland:  os.Getenv("WAYLAND_DISPLAY") != "",
		remote:   remoteSession(os.Getenv),
		terminal: copyTerminal,
		native:   clipboard.WriteAll,
		lookPath: exec.LookPath,
		run: func(name string, args []string, text string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Stdin = strings.NewReader(text)
			err := cmd.Run()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		},
	})
}

func copyWith(text string, deps dependencies) error {
	var failures []error
	tryTerminal := func() error {
		if deps.terminal == nil {
			return errTerminalUnavailable
		}
		err := deps.terminal(text)
		if err != nil {
			cause := debug.IO
			if errors.Is(err, errTerminalUnavailable) {
				cause = debug.Unavailable
			}
			if errors.Is(err, errTerminalLimit) {
				cause = debug.Invalid
			}
			debug.Failure("clipboard osc52", cause)
		}
		return err
	}
	// Host clipboard utilities in an SSH session target the wrong machine.
	if deps.remote {
		return tryTerminal()
	}
	tryCommand := func(name string, args ...string) bool {
		operation := debug.Operation("clipboard xclip")
		if name == "wl-copy" {
			operation = "clipboard wl-copy"
		}
		if _, err := deps.lookPath(name); err != nil {
			debug.Failure(operation, debug.Unavailable)
			return false
		}
		runErr := deps.run(name, args, text)
		if runErr == nil {
			return true
		}
		cause := debug.Clipboard
		if errors.Is(runErr, context.DeadlineExceeded) {
			cause = debug.Timeout
		}
		debug.Failure(operation, cause)
		failures = append(failures, fmt.Errorf("%s: %w", name, runErr))
		return false
	}

	// wl-copy is the native Wayland path and avoids depending on a toolkit.
	if deps.wayland && tryCommand("wl-copy") {
		return nil
	}
	if err := deps.native(text); err == nil {
		return nil
	} else {
		debug.Failure("clipboard native", debug.Clipboard)
		failures = append(failures, fmt.Errorf("native clipboard: %w", err))
	}
	if !deps.wayland && tryCommand("wl-copy") {
		return nil
	}
	if tryCommand("xclip", "-selection", "clipboard") {
		return nil
	}
	if deps.terminal != nil {
		if err := tryTerminal(); err == nil {
			return nil
		} else {
			failures = append(failures, err)
		}
	}
	if len(failures) == 0 {
		return errors.New("no supported clipboard program found")
	}
	return fmt.Errorf("copying to clipboard failed: %w", errors.Join(failures...))
}

func remoteSession(getenv func(string) string) bool {
	for _, key := range []string{"SSH_CONNECTION", "SSH_CLIENT", "SSH_TTY", "MOSH_CONNECTION"} {
		if getenv(key) != "" {
			return true
		}
	}
	return false
}
