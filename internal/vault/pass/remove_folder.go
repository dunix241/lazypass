package pass

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"lazypass/internal/vault"
	"lazypass/internal/vault/unfiled"
)

func (p *Provider) RemoveFolder(ctx context.Context, path vault.Path) error {
	if _, err := p.safeDirectory(path); err != nil {
		return err
	}
	store, err := p.resolvedStoreDir()
	if err != nil {
		return err
	}
	r, err := os.OpenRoot(store)
	if err != nil {
		return err
	}
	defer r.Close()

	moves, err := unfiled.Plan(ctx, r, path, ".gpg")
	if err != nil {
		return err
	}
	if err := p.CreateFolder(ctx, vault.Path{unfiled.Directory}); err != nil {
		return err
	}
	cleanup, err := p.keepDeleteParent(path)
	if err != nil {
		return err
	}
	defer cleanup()

	for _, move := range moves {
		if err := ctx.Err(); err != nil {
			return &unfiled.MoveFailure{Err: err}
		}
		if _, err := p.run(ctx, []string{"mv", "--", move.Source.String(), move.Destination.String()}, nil); err != nil {
			return &unfiled.MoveFailure{Err: err}
		}
	}

	if err := unfiled.RemapMetadata(store, moves, func(path vault.Path) ([]vault.Node, error) { return p.List(ctx, path) }); err != nil {
		return unfiled.MetadataWarning()
	}

	// Only the recipient-policy file may remain; never recursively remove leftovers.
	if err := p.removeFolderPolicy(ctx, r, store, path); err != nil {
		return err
	}
	if err := r.Remove(path.String()); err != nil && !os.IsNotExist(err) {
		return &unfiled.MoveFailure{Err: err}
	}
	if err := p.commitMetadata(ctx); err != nil {
		return vault.SavedWarning{}
	}
	return nil
}

func (p *Provider) removeFolderPolicy(ctx context.Context, r *os.Root, store string, path vault.Path) error {
	policy := path.String() + "/.gpg-id"
	if _, err := r.Lstat(policy); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return &unfiled.MoveFailure{Err: err}
	}
	if err := r.Remove(policy); err != nil {
		return &unfiled.MoveFailure{Err: err}
	}

	if _, err := os.Stat(filepath.Join(store, ".git")); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if _, err := p.run(ctx, []string{"git", "add", "-A", "--", policy}, nil); err != nil {
		return vault.SavedWarning{}
	}
	changes, err := p.run(ctx, []string{"git", "diff", "--cached", "--name-only", "--", policy}, nil)
	if err != nil {
		return vault.SavedWarning{}
	}
	if len(changes) == 0 {
		return nil
	}
	if _, err := p.run(ctx, []string{"git", "commit", "-m", "Remove empty folder recipient policy", "--", policy}, nil); err != nil {
		return vault.SavedWarning{}
	}
	return nil
}
