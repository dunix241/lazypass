package install

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func fixtureClient(script string, corrupt bool) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := script
		if strings.HasSuffix(req.URL.Path, "/lazypass_checksum_sha256.txt") {
			hash := sha256.Sum256([]byte(script))
			if corrupt {
				hash = [32]byte{}
			}
			body = fmt.Sprintf("%x  install.sh\n", hash)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
}

func TestDelegateArgumentsAndVerification(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("POSIX installer")
	}
	dir := filepath.Join(t.TempDir(), `bin space;$(touch injected)`)
	owner := Ownership{Executable: filepath.Join(dir, "lazypass"), Method: "manual"}
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\"\n"
	var out bytes.Buffer
	if err := delegate(context.Background(), owner, "v1.2.3", &out, io.Discard, fixtureClient(script, false)); err != nil {
		t.Fatal(err)
	}
	want := "--version\nv1.2.3\n--install-dir\n" + dir + "\n"
	if out.String() != want {
		t.Fatalf("argument boundaries: got %q, want %q", out.String(), want)
	}
	out.Reset()
	if err := delegate(context.Background(), owner, "v1.2.3", &out, io.Discard, fixtureClient(script, true)); err == nil || out.Len() != 0 {
		t.Fatalf("unverified script executed: %q %v", out.String(), err)
	}
}

func TestDelegateCancellationCleansInstaller(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("POSIX cancellation")
	}
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	cleaned := filepath.Join(dir, "cleaned")
	script := fmt.Sprintf("#!/bin/sh\ntrap 'touch %s; exit 143' TERM\ntouch %s\nsleep 60 &\nwait\n", cleaned, ready)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- delegate(ctx, Ownership{Executable: filepath.Join(dir, "lazypass"), Method: "script"}, "v1.0.0", io.Discard, io.Discard, fixtureClient(script, false))
	}()
	deadline := time.After(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("installer exited before cancellation: %v", err)
		case <-deadline:
			t.Fatal("installer did not start")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled installer succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("installer did not stop")
	}
	if _, err := os.Stat(cleaned); err != nil {
		t.Fatalf("installer cleanup trap did not run: %v", err)
	}
}

func TestFetchRejectsFailedAndOversizedResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"not found", http.StatusNotFound, "missing"},
		{"oversized", http.StatusOK, "12345"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			if _, err := fetch(context.Background(), client, releasesURL+"fixture", 4); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}
