package pass

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"lazypass/internal/vault"
	"lazypass/internal/vault/metadata"
)

// CreateFolder creates metadata-only directories; encryption remains owned by pass.
// Empty directories are local metadata and are not tracked by Git.
func (p *Provider) CreateFolder(ctx context.Context, path vault.Path) error {
	if err := ctx.Err(); err != nil {
		return vault.ErrUnavailable
	}
	if err := requireEntryPath(path); err != nil {
		return err
	}
	root, err := p.resolvedStoreDir()
	if err != nil {
		return err
	}
	if info, err := os.Lstat(filepath.Join(root, ".gpg-id")); err != nil || !info.Mode().IsRegular() {
		return vault.ErrUninitialized
	}
	for _, part := range path {
		if err := ctx.Err(); err != nil {
			return vault.ErrUnavailable
		}
		root = filepath.Join(root, part)
		if err := ensureDirectory(root); err != nil {
			return err
		}
	}
	return nil
}

func ensureDirectory(path string) error {
	if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return vault.ErrUnavailable
	}
	info, err := os.Lstat(path)
	if err != nil {
		return vault.ErrUnavailable
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return vault.ErrInvalidPath
	}
	if !info.IsDir() {
		return vault.ErrConflict
	}
	return nil
}

func (p *Provider) DeleteFolder(ctx context.Context, path vault.Path) error {
	if err := requireEntryPath(path); err != nil {
		return err
	}
	if _, err := p.safeDirectory(path); err != nil {
		return err
	}
	cleanup, err := p.keepDeleteParent(path)
	if err != nil {
		return err
	}
	defer cleanup()
	_, err = p.run(ctx, []string{"rm", "--recursive", "--force", "--", path.String() + "/"}, nil)
	if err == nil {
		if metaErr := metadata.Remove(p.storeDir(), path, true); metaErr == nil {
			_ = p.commitMetadata(ctx)
		}
	}
	return err
}

// pass rm prunes empty ancestors with rmdir -p. A temporary metadata file in
// the containing folder prevents pruning while leaving deletion/Git to pass.
func (p *Provider) keepDeleteParent(path vault.Path) (func(), error) {
	parent, err := p.safeDirectory(path[:len(path)-1])
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(parent, ".lazypass-keep-*")
	if err != nil {
		return nil, vault.ErrUnavailable
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		os.Remove(name)
		return nil, vault.ErrUnavailable
	}
	return func() { _ = os.Remove(name) }, nil
}
