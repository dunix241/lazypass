// Package util provides terminal detection and private file writes.
package util

import (
	"golang.org/x/term"
	"os"
)

// IsTerminal reports whether f is an actual terminal, rather than any character device.
func IsTerminal(f *os.File) bool {
	return f != nil && term.IsTerminal(int(f.Fd()))
}
