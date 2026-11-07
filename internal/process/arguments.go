// Package process handles platform-specific application launch conventions.
package process

import (
	"os"
	"path/filepath"
	"runtime"
)

// ApplicationArgs removes Android's direct-linker prefix, which libc normally
// skips but a pure-Go executable receives in its original argument stack.
func ApplicationArgs(goos string, args []string) []string {
	if goos != "android" || len(args) < 2 || !isAndroidLinker(args[0]) || !filepath.IsAbs(args[1]) {
		return args
	}
	return args[1:]
}

// Executable returns the application rather than the Android linker when the
// launcher is exposed by /proc/self/exe. Arguments may already be normalized.
func Executable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return applicationExecutable(runtime.GOOS, path, os.Args), nil
}

func applicationExecutable(goos, path string, args []string) string {
	if goos != "android" || !isAndroidLinker(path) {
		return path
	}
	args = ApplicationArgs(goos, args)
	if len(args) > 0 && filepath.IsAbs(args[0]) && !isAndroidLinker(args[0]) {
		return args[0]
	}
	return path
}

func isAndroidLinker(path string) bool {
	switch path {
	case "/system/bin/linker", "/system/bin/linker64", "/apex/com.android.runtime/bin/linker", "/apex/com.android.runtime/bin/linker64":
		return true
	default:
		return false
	}
}
