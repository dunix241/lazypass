package process

import (
	"reflect"
	"testing"
)

func TestMeasuredTermuxLaunchLayouts(t *testing.T) {
	app := "/data/data/com.termux/files/home/termux-launch-probe"
	linker := "/apex/com.android.runtime/bin/linker64"
	for _, tc := range []struct {
		name, executable, marker string
		args, want               []string
	}{
		{"cross compiled ordinary", linker, app, []string{app, app, "--help"}, []string{app, "--help"}},
		{"cross compiled direct", linker, "", []string{"/system/bin/linker64", app, "--help"}, []string{app, "--help"}},
		{"native ordinary", app, app, []string{app, "--help"}, []string{app, "--help"}},
		{"native direct", app, "", []string{"/system/bin/linker64", "--help"}, []string{"/system/bin/linker64", "--help"}},
		{"native path argument", app, app, []string{app, app}, []string{app, app}},
		{"unmatched marker", linker, "/other", []string{app, app}, []string{app, app}},
		{"ordinary no arguments", linker, app, []string{app, app}, []string{app}},
		{"direct no arguments", linker, "", []string{linker, app}, []string{app}},
		{"subcommand", linker, app, []string{app, app, "update", "check"}, []string{app, "update", "check"}},
		{"missing program", linker, "", []string{linker}, []string{linker}},
		{"relative program", linker, "", []string{linker, "lazypass"}, []string{linker, "lazypass"}},
		{"empty arguments", linker, "", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := launchArgs(tc.executable, tc.marker, tc.args); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestApplicationArgsLeavesOtherPlatformsUnchanged(t *testing.T) {
	args := []string{"/system/bin/linker64", "/example/app", "--help"}
	if got := ApplicationArgs("linux", args); !reflect.DeepEqual(got, args) {
		t.Fatalf("non-Android arguments changed: %q", got)
	}
}

func TestApplicationExecutable(t *testing.T) {
	app := "/data/data/com.termux/files/home/.local/bin/lazypass"
	linker := "/system/bin/linker64"
	for _, tc := range []struct {
		name, goos, executable string
		args                   []string
		want                   string
	}{
		{"raw launcher", "android", linker, []string{linker, app, "--help"}, app},
		{"normalized launcher", "android", linker, []string{app, "--help"}, app},
		{"native executable", "android", app, []string{app}, app},
		{"missing arguments", "android", linker, nil, linker},
		{"relative argument", "android", linker, []string{"lazypass"}, linker},
		{"other platform", "linux", linker, []string{app}, linker},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := applicationExecutable(tc.goos, tc.executable, tc.args); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
