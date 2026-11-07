package process

import (
	"reflect"
	"testing"
)

func TestApplicationArgs(t *testing.T) {
	app := "/data/data/com.termux/files/home/.local/bin/lazypass"
	for _, tc := range []struct {
		name, os    string
		input, want []string
	}{
		{"linker launch", "android", []string{"/system/bin/linker64", app}, []string{app}},
		{"help", "android", []string{"/system/bin/linker64", app, "--help"}, []string{app, "--help"}},
		{"subcommand", "android", []string{"/apex/com.android.runtime/bin/linker64", app, "update", "check"}, []string{app, "update", "check"}},
		{"native launch", "android", []string{app, "--help"}, []string{app, "--help"}},
		{"ordinary argument", "android", []string{app, app}, []string{app, app}},
		{"other platform", "linux", []string{"/system/bin/linker64", app}, []string{"/system/bin/linker64", app}},
		{"missing program", "android", []string{"/system/bin/linker64"}, []string{"/system/bin/linker64"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ApplicationArgs(tc.os, tc.input); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
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
