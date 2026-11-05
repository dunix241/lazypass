package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func write(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func missing(context.Context, string, ...string) error {
	return &exec.Error{Name: "fixture", Err: exec.ErrNotFound}
}

func TestOwnership(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "lazypass")
	data := []byte("fixture binary")
	write(t, binary, data, 0700)
	binary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	receipt := Receipt{Version: 1, Method: "script", Executable: binary, Release: "v1.0.0", SHA256: hex.EncodeToString(hash[:])}
	raw, _ := json.Marshal(receipt)
	path := filepath.Join(dir, "install.json")
	write(t, path, raw, 0600)
	owner, err := Inspect(context.Background(), binary, path, "linux", missing)
	if err != nil || owner.Method != "script" {
		t.Fatalf("%+v %v", owner, err)
	}
	owner, err = Inspect(context.Background(), binary, path, "linux", func(context.Context, string, ...string) error { return nil })
	if err != nil || owner.Method != "package" {
		t.Fatalf("package must beat receipt: %+v %v", owner, err)
	}
	write(t, binary, []byte("changed"), 0700)
	owner, err = Inspect(context.Background(), binary, path, "linux", missing)
	if err != nil || owner.Method != "manual" {
		t.Fatalf("changed digest: %+v %v", owner, err)
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link")
		if err := os.Symlink(binary, link); err != nil {
			t.Fatal(err)
		}
		owner, err = Inspect(context.Background(), link, path, "linux", missing)
		if err != nil || owner.Executable != binary {
			t.Fatalf("symlink: %+v %v", owner, err)
		}
	}
}

func TestVerify(t *testing.T) {
	data := []byte("installer")
	hash := sha256.Sum256(data)
	line := fmt.Sprintf("%x  install.sh\n", hash)
	for _, tc := range []struct {
		name, checks string
		ok           bool
	}{
		{"valid", line, true}, {"duplicate", line + line, false}, {"missing", "", false}, {"mismatch", strings.Repeat("0", 64) + "  install.sh\n", false}, {"extra field", strings.TrimSpace(line) + " extra", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if (verifyChecksum(data, []byte(tc.checks), "install.sh") == nil) != tc.ok {
				t.Fatal("unexpected verification result")
			}
		})
	}
}

func TestOwnershipQueryFailureBlocksAdoption(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "lazypass")
	write(t, binary, []byte("fixture"), 0700)
	_, err := Inspect(context.Background(), binary, filepath.Join(dir, "missing.json"), "linux", func(context.Context, string, ...string) error {
		return context.DeadlineExceeded
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("query failure must block ownership determination: %v", err)
	}
}

func archive(t *testing.T, path string, kind byte) []byte {
	t.Helper()
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	tw := tar.NewWriter(gz)
	data := []byte("fixture executable - not executed")
	h := &tar.Header{Name: "lazypass", Mode: 0755, Size: int64(len(data)), Typeflag: kind}
	if kind == tar.TypeSymlink {
		h.Size = 0
		h.Linkname = "/etc/passwd"
	}
	if err := tw.WriteHeader(h); err != nil {
		t.Fatal(err)
	}
	if kind == tar.TypeReg {
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	write(t, path, b.Bytes(), 0600)
	return b.Bytes()
}

func TestStandaloneInstaller(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer")
	}
	script, err := filepath.Abs("../../scripts/install.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"install-update", "quoted-path", "checksum", "duplicate", "malformed-checksum", "hash-failure", "symlink-archive", "package-owner", "query-failure", "unknown", "destination-symlink", "network"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			dir, err := filepath.EvalSymlinks(dir)
			if err != nil {
				t.Fatal(err)
			}
			tools := filepath.Join(dir, "tools")
			if err := os.Mkdir(tools, 0700); err != nil {
				t.Fatal(err)
			}
			fixtureOS := map[string]string{"linux": "Linux", "darwin": "Darwin"}[runtime.GOOS]
			fixtureArch := map[string]string{"amd64": "x86_64", "arm64": "arm64"}[runtime.GOARCH]
			if scenario == "package-owner" || scenario == "query-failure" {
				fixtureOS = "Linux"
				write(t, filepath.Join(tools, "uname"), []byte(fmt.Sprintf(`#!/bin/sh
case "$1" in
    -s) printf 'Linux\n' ;;
    -m) printf '%s\n' ;;
esac
`, fixtureArch)), 0700)
			}
			// Intercept downloads, retaining production URL and TLS arguments.
			write(t, filepath.Join(tools, "curl"), []byte(`#!/bin/sh
set -eu
out=
while [ "$#" -gt 0 ]; do
 case "$1" in --output) out=$2; shift 2;; *) url=$1; shift;; esac
done
[ "${FIXTURE_FAIL:-}" != network ] || exit 22
case "$url" in *.tar.gz) cp "$FIXTURE/archive" "$out";; *sha256.txt) cp "$FIXTURE/checksums" "$out";; *) exit 22;; esac
`), 0700)
			for _, tool := range []string{"pacman", "dpkg-query", "rpm", "apk"} {
				code := "#!/bin/sh\nexit 1\n"
				if scenario == "package-owner" && tool == "pacman" {
					code = "#!/bin/sh\nexit 0\n"
				}
				if scenario == "query-failure" && tool == "pacman" {
					code = "#!/bin/sh\nexit 2\n"
				}
				write(t, filepath.Join(tools, tool), []byte(code), 0700)
			}
			if scenario == "hash-failure" {
				write(t, filepath.Join(tools, "sha256sum"), []byte("#!/bin/sh\nexit 1\n"), 0700)
			}
			kind := byte(tar.TypeReg)
			if scenario == "symlink-archive" {
				kind = tar.TypeSymlink
			}
			data := archive(t, filepath.Join(dir, "archive"), kind)
			hash := sha256.Sum256(data)
			if scenario == "checksum" {
				hash = [32]byte{}
			}
			checks := fmt.Sprintf("%x  lazypass_%s_%s.tar.gz\n", hash, fixtureOS, fixtureArch)
			if scenario == "duplicate" {
				checks += checks
			}
			if scenario == "malformed-checksum" {
				checks = strings.TrimSpace(checks) + " extra\n"
			}
			write(t, filepath.Join(dir, "checksums"), []byte(checks), 0600)
			bin := filepath.Join(dir, "bin with spaces")
			if scenario == "quoted-path" {
				bin = filepath.Join(dir, `bin "quoted" with \slashes`)
			}
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(bin, "lazypass")
			old := []byte("old binary survives failure")
			success := scenario == "install-update" || scenario == "quoted-path"
			if !success {
				write(t, target, old, 0700)
			}
			if scenario == "destination-symlink" {
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(dir, "archive"), target); err != nil {
					t.Fatal(err)
				}
			}
			runInstall := func(version string) ([]byte, error) {
				args := []string{script, "--version", version, "--install-dir", bin}
				if !success && scenario != "unknown" {
					args = append(args, "--adopt")
				}
				cmd := exec.Command("sh", args...)
				cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"), "HOME="+dir, "XDG_STATE_HOME="+filepath.Join(dir, "state"), "FIXTURE="+dir, "FIXTURE_FAIL="+scenario)
				return cmd.CombinedOutput()
			}
			out, err := runInstall("v1.0.0")
			if success {
				if err != nil {
					t.Fatalf("%v: %s", err, out)
				}
				out, err = runInstall("v1.1.0")
				if err != nil {
					t.Fatalf("upgrade %v: %s", err, out)
				}
				owner, err := Inspect(context.Background(), target, filepath.Join(dir, "state", "lazypass", "install.json"), "linux", missing)
				if err != nil || owner.Method != "script" || owner.Release != "v1.1.0" {
					t.Fatalf("receipt: %+v %v", owner, err)
				}
				if out, err = runInstall("v1.0.0"); err == nil {
					t.Fatalf("downgrade allowed: %s", out)
				}
			} else {
				if err == nil {
					t.Fatalf("expected failure: %s", out)
				}
				if scenario != "destination-symlink" {
					got, err := os.ReadFile(target)
					if err != nil || !bytes.Equal(got, old) {
						t.Fatalf("old executable changed: %q %v", got, err)
					}
				}
			}
			staged, err := filepath.Glob(filepath.Join(bin, ".lazypass*"))
			if err != nil || len(staged) != 0 {
				t.Fatalf("installer left staging files or lock: %v %v", staged, err)
			}
		})
	}
}
