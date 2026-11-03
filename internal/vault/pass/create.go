package pass

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"lazypass/internal/vault"
)

// Create encrypts in an isolated store, then atomically publishes the ciphertext.
// Hard linking on the same filesystem fails rather than replacing a competing entry.
func (p *Provider) Create(ctx context.Context, entry vault.Entry) error {
	if ctx.Err() != nil {
		return vault.ErrUnavailable
	}
	if err := requireEntryPath(entry.Path); err != nil {
		return err
	}
	encoded, err := vault.EncodeEntry(entry)
	if err != nil {
		return err
	}
	root, err := p.resolvedStoreDir()
	if err != nil {
		return err
	}
	parent := root
	for _, part := range entry.Path[:len(entry.Path)-1] {
		parent = filepath.Join(parent, part)
		if err := ensureDirectory(parent); err != nil {
			return err
		}
	}
	target := filepath.Join(parent, entry.Path[len(entry.Path)-1]+".gpg")
	if _, err := os.Lstat(target); err == nil {
		return vault.ErrConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return vault.ErrUnavailable
	}
	recipientDir, err := recipientDirectory(root, parent)
	if err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".lazypass-create-")
	if err != nil {
		return vault.ErrUnavailable
	}
	defer os.RemoveAll(stage)
	for _, name := range []string{".gpg-id", ".gpg-id.sig"} {
		if err := copyRecipientFile(recipientDir, stage, name); err != nil {
			return err
		}
	}
	staged := New(stage, p.Runner)
	staged.disableGit = true
	if _, err := staged.run(ctx, []string{"insert", "--multiline", "--force", "--", "entry"}, strings.NewReader(encoded)); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return vault.ErrUnavailable
	}
	if err := os.Link(filepath.Join(stage, "entry.gpg"), target); err != nil {
		if errors.Is(err, os.ErrExist) {
			return vault.ErrConflict
		}
		return vault.ErrUnavailable
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		if _, err := p.run(ctx, []string{"git", "add", "--", entry.Path.String() + ".gpg"}, nil); err != nil {
			return err
		}
		if _, err := p.run(ctx, []string{"git", "commit", "-m", "Add vault entry", "--", entry.Path.String() + ".gpg"}, nil); err != nil {
			return err
		}
	}
	if err := p.WriteMetadata(ctx, entry.Path, entry.Type); err != nil {
		return vault.SavedWarning{}
	}
	return nil
}

func recipientDirectory(root, parent string) (string, error) {
	for dir := parent; ; dir = filepath.Dir(dir) {
		info, err := os.Lstat(filepath.Join(dir, ".gpg-id"))
		if err == nil {
			if !info.Mode().IsRegular() {
				return "", vault.ErrInvalidPath
			}
			return dir, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", vault.ErrUnavailable
		}
		if dir == root {
			return "", vault.ErrUninitialized
		}
	}
}

func copyRecipientFile(source, destination, name string) error {
	path := filepath.Join(source, name)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && name == ".gpg-id.sig" {
		return nil
	}
	if err != nil {
		return vault.ErrUnavailable
	}
	if !info.Mode().IsRegular() {
		return vault.ErrInvalidPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return vault.ErrUnavailable
	}
	if err := os.WriteFile(filepath.Join(destination, name), data, 0600); err != nil {
		return vault.ErrUnavailable
	}
	return nil
}
