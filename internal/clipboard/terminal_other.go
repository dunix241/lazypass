//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package clipboard

import "io"

func openTerminal() (io.WriteCloser, error) { return nil, errTerminalUnavailable }
