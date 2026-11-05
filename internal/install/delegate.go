package install

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lazypass/internal/update"
)

const releasesURL = "https://github.com/dunix241/lazypass/releases/download/"

// Automatic installs only verified script-owned executables. Other installation
// methods fall back to notification without adoption or elevated privileges.
func Automatic(ctx context.Context, version string) (bool, error) {
	owner, err := Detect(ctx)
	if err != nil {
		return false, err
	}
	if owner.Method != "script" || owner.ValidateUpdate(false) != nil {
		return false, nil
	}
	if !update.Newer(version, owner.Release) {
		return true, nil
	}
	return true, Delegate(ctx, owner, version, false, io.Discard, io.Discard)
}

// ValidateUpdate checks eligibility before any release downloads.
func (o Ownership) ValidateUpdate(adopt bool) error {
	switch {
	case runtime.GOOS != "linux" && runtime.GOOS != "darwin":
		return errors.New("standalone updates require Linux or macOS; use your package manager or release download")
	case o.Method == "package" || o.Method == "protected":
		return errors.New(o.Guidance)
	case o.Method != "script" && !adopt:
		return fmt.Errorf("unrecognized installation at %s; use lazypass update --adopt to explicitly adopt it, or update using your original installation method", o.Executable)
	case !filepath.IsAbs(o.Executable) || filepath.Base(o.Executable) != "lazypass":
		return errors.New("standalone updates require an absolute executable path named lazypass")
	}
	return nil
}

// Delegate verifies the release script before execution. The trust root is HTTPS
// to GitHub; the accompanying checksum is not an independent signature.
func Delegate(ctx context.Context, owner Ownership, version string, adopt bool, stdout, stderr io.Writer) error {
	return delegate(ctx, owner, version, adopt, stdout, stderr, installerClient())
}

func installerClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || len(via) >= 10 {
				return errors.New("unsafe installer redirect")
			}
			return nil
		},
	}
}

func delegate(ctx context.Context, owner Ownership, version string, adopt bool, stdout, stderr io.Writer, client *http.Client) error {
	if err := owner.ValidateUpdate(adopt); err != nil {
		return err
	}
	if !update.Stable(version) {
		return errors.New("expected stable version vX.Y.Z")
	}
	version = "v" + strings.TrimPrefix(version, "v")
	base := releasesURL + version + "/"
	checksums, err := fetch(ctx, client, base+"lazypass_checksum_sha256.txt", 64<<10)
	if err != nil {
		return fmt.Errorf("download installer checksums: %w", err)
	}
	script, err := fetch(ctx, client, base+"install.sh", 256<<10)
	if err != nil {
		return fmt.Errorf("download installer (this release may predate standalone updates): %w", err)
	}
	if err := verifyChecksum(script, checksums, "install.sh"); err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "lazypass-install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "install.sh")
	if err := os.WriteFile(path, script, 0600); err != nil {
		return err
	}
	args := []string{path, "--version", version, "--install-dir", filepath.Dir(owner.Executable)}
	if adopt {
		args = append(args, "--adopt")
	}
	cmd := exec.CommandContext(ctx, "sh", args...)
	configureCancellation(cmd)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	return cmd.Run()
}

func fetch(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("installer response exceeds size limit")
	}
	return data, nil
}

func verifyChecksum(data, checksums []byte, name string) error {
	count := 0
	digest := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == name {
			count++
			if len(fields) != 2 {
				return errors.New("malformed installer checksum")
			}
			digest = fields[0]
		}
	}
	hash := sha256.Sum256(data)
	if count != 1 || digest != hex.EncodeToString(hash[:]) {
		return errors.New("missing, duplicate, or mismatched installer checksum")
	}
	return nil
}
