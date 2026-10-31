//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package clipboard

import (
	"io"
	"os"

	"golang.org/x/term"
)

func openTerminal() (io.WriteCloser, error) {
	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return nil, errTerminalUnavailable
	}
	if !term.IsTerminal(int(f.Fd())) {
		f.Close()
		return nil, errTerminalUnavailable
	}
	return f, nil
}
