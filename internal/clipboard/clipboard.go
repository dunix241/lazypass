// Package clipboard copies text to the system clipboard without exposing it to
// stdout. Wayland is preferred when available because lazypass targets Hyprland.
package clipboard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

type dependencies struct {
	wayland  bool
	native   func(string) error
	lookPath func(string) (string, error)
	run      func(string, []string, string) error
}

// Copy writes text to the best available clipboard destination. It returns an
// error only after every available destination has failed.
func Copy(text string) error {
	return copyWith(text, dependencies{
		wayland:  os.Getenv("WAYLAND_DISPLAY") != "",
		native:   clipboard.WriteAll,
		lookPath: exec.LookPath,
		run: func(name string, args []string, text string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		},
	})
}

func copyWith(text string, deps dependencies) error {
	var failures []error
	tryCommand := func(name string, args ...string) bool {
		if _, err := deps.lookPath(name); err != nil {
			return false
		}
		runErr := deps.run(name, args, text)
		if runErr == nil {
			return true
		}
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
		failures = append(failures, fmt.Errorf("native clipboard: %w", err))
	}
	if !deps.wayland && tryCommand("wl-copy") {
		return nil
	}
	if tryCommand("xclip", "-selection", "clipboard") {
		return nil
	}
	if len(failures) == 0 {
		return errors.New("no supported clipboard program found")
	}
	return fmt.Errorf("copying to clipboard failed: %w", errors.Join(failures...))
}
