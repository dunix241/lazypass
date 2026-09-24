// Package util holds XDG paths and TTY detection.
package util

import "os"

// IsTerminal reports whether f is a character device (TTY).
func IsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
