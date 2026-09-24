package clipboard

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestWaylandPrefersWlCopy(t *testing.T) {
	var calls []string
	err := copyWith("secret", dependencies{
		wayland:  true,
		native:   func(string) error { calls = append(calls, "native"); return nil },
		lookPath: func(name string) (string, error) { return name, nil },
		run: func(name string, _ []string, text string) error {
			if text != "secret" {
				t.Fatalf("got %q", text)
			}
			calls = append(calls, name)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"wl-copy"}) {
		t.Fatalf("calls = %v, want wl-copy only", calls)
	}
}

func TestFallsBackFromWaylandToNativeThenXclip(t *testing.T) {
	var calls []string
	err := copyWith("secret", dependencies{
		wayland:  true,
		native:   func(string) error { calls = append(calls, "native"); return errors.New("unavailable") },
		lookPath: func(name string) (string, error) { return name, nil },
		run: func(name string, _ []string, _ string) error {
			calls = append(calls, name)
			if name == "wl-copy" {
				return errors.New("failed")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"wl-copy", "native", "xclip"}) {
		t.Fatalf("calls = %v, want wl-copy, native, xclip", calls)
	}
}

func TestCopyReportsAllAvailableFailures(t *testing.T) {
	err := copyWith("secret", dependencies{
		wayland:  false,
		native:   func(string) error { return errors.New("native failed") },
		lookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		run:      func(string, []string, string) error { t.Fatal("run should not be called"); return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "native failed") {
		t.Fatalf("got %v, want native failure", err)
	}
}
