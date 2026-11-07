// Package install detects installation ownership and delegates explicit upgrades
// to the standalone release installer. It never replaces executables itself.
package install

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lazypass/internal/process"
	"lazypass/internal/update"
)

type Receipt struct {
	Version    int    `json:"version"`
	Method     string `json:"method"`
	Executable string `json:"executable"`
	Release    string `json:"release"`
	SHA256     string `json:"sha256"`
}

type Ownership struct {
	Executable string
	Method     string
	Guidance   string
	Release    string
}

// Runner returns exit code 1 for a package query with no owner. Other failures
// are deliberately not interpreted as proof of an unmanaged installation.
type Runner func(context.Context, string, ...string) error

func run(ctx context.Context, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Run()
}

func ReceiptPath() (string, error) {
	root := os.Getenv("XDG_STATE_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".local", "state")
	}
	if !filepath.IsAbs(root) {
		return "", errors.New("XDG_STATE_HOME must be absolute")
	}
	return filepath.Join(root, "lazypass", "install.json"), nil
}

func Detect(ctx context.Context) (Ownership, error) {
	path, err := process.Executable()
	if err != nil {
		return Ownership{}, err
	}
	receipt, err := ReceiptPath()
	if err != nil {
		return Ownership{}, err
	}
	return Inspect(ctx, path, receipt, runtime.GOOS, run)
}

// Inspect resolves the supplied executable, independently of PATH.
func Inspect(ctx context.Context, path, receiptPath, goos string, runner Runner) (Ownership, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return Ownership{}, err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return Ownership{}, err
	}
	result := Ownership{Executable: path, Method: "manual"}
	if strings.HasPrefix(path, "/nix/store/") {
		result.Method, result.Guidance = "package", "Update lazypass through your Nix configuration."
		return result, nil
	}
	if goos == "windows" {
		result.Guidance = "Update using your package manager or https://github.com/dunix241/lazypass/releases/latest."
		return result, nil
	}
	if goos == "darwin" && strings.Contains(path, "/Cellar/") {
		result.Method, result.Guidance = "protected", "This destination is in a Cellar directory; update through its original installation method."
		return result, nil
	}
	if goos == "linux" || goos == "android" {
		queries := []struct {
			name     string
			args     []string
			guidance string
		}{
			{"pacman", []string{"-Qoq", path}, "Update with your Arch/AUR package manager (for example: yay -Syu)."},
			{"dpkg-query", []string{"-S", path}, "Update lazypass with your Debian package manager."},
			{"rpm", []string{"-qf", path}, "Update lazypass with your RPM package manager."},
			{"apk", []string{"info", "--who-owns", path}, "Update with your Alpine package manager: apk upgrade"},
		}
		for _, q := range queries {
			err := runner(ctx, q.name, q.args...)
			if err == nil {
				result.Method, result.Guidance = "package", q.guidance
				return result, nil
			}
			var missing *exec.Error
			if errors.As(err, &missing) && errors.Is(missing.Err, exec.ErrNotFound) {
				continue
			}
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 1 {
				continue
			}
			return Ownership{}, fmt.Errorf("cannot establish package ownership (%s): %w", q.name, err)
		}
	}
	receipt, err := readReceipt(receiptPath)
	if err != nil {
		return result, err
	}
	if receipt == nil || receipt.Executable != path {
		return result, nil
	}
	digest, err := fileDigest(path)
	if err != nil {
		return result, err
	}
	if receipt.SHA256 == digest {
		result.Method, result.Release = "script", receipt.Release
	}
	return result, nil
}

func readReceipt(path string) (*Receipt, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read installation receipt: %w", err)
	}
	defer f.Close()
	var receipt Receipt
	data, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil {
		return nil, err
	}
	if len(data) > 8192 || json.Unmarshal(data, &receipt) != nil || receipt.Version != 1 || receipt.Method != "script" || !update.Stable(receipt.Release) {
		return nil, nil
	}
	return &receipt, nil
}

func fileDigest(path string) (string, error) {
	binary, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer binary.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, binary); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
