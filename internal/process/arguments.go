// Package process handles platform-specific application launch conventions.
package process

import (
	"os"
	"path/filepath"
	"runtime"
)

// ApplicationArgs compensates for Android linker arguments only when the runtime
// still identifies the linker as the executable. Termux-patched runtimes already
// normalize these arguments and must be left untouched.
func ApplicationArgs(goos string, args []string) []string {
	if goos != "android" {
		return args
	}
	executable, err := os.Executable()
	if err != nil {
		return args
	}
	return launchArgs(executable, os.Getenv("TERMUX_EXEC__PROC_SELF_EXE"), args)
}

func launchArgs(executable, termuxExecutable string, args []string) []string {
	if !isAndroidLinker(executable) || len(args) < 2 || !filepath.IsAbs(args[1]) {
		return args
	}
	markedExecutable := filepath.IsAbs(termuxExecutable) && filepath.Clean(args[1]) == filepath.Clean(termuxExecutable)
	if isAndroidLinker(args[0]) || markedExecutable {
		return args[1:]
	}
	return args
}

// Executable returns the application rather than the Android linker. Call after
// normalizing os.Args with ApplicationArgs.
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
	if len(args) > 0 && isAndroidLinker(args[0]) {
		args = launchArgs(path, "", args)
	}
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
