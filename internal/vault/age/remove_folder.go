package age

import (
	"context"
	"os"

	"lazypass/internal/vault"
	"lazypass/internal/vault/unfiled"
)

func (p *Provider) RemoveFolder(ctx context.Context, path vault.Path) error {
	r, err := p.open(ctx)
	if err != nil {
		return err
	}
	defer r.Close()

	if _, err := readManifest(r); err != nil {
		return err
	}

	moves, err := unfiled.Plan(ctx, r, path, ".age")
	if err != nil {
		return err
	}
	if err := r.Mkdir(unfiled.Directory, 0700); err != nil && !os.IsExist(err) {
		return err
	}

	for _, move := range moves {
		if err := ctx.Err(); err != nil {
			return &unfiled.MoveFailure{Err: err}
		}
		from, to := move.Source.String(), move.Destination.String()
		if !move.Folder {
			from += ".age"
			to += ".age"
		}
		if err := r.Rename(from, to); err != nil {
			return &unfiled.MoveFailure{Err: err}
		}
	}

	if err := r.Remove(path.String()); err != nil {
		return &unfiled.MoveFailure{Err: err}
	}
	if err := unfiled.RemapMetadata(p.StoreDir, moves, func(path vault.Path) ([]vault.Node, error) { return p.List(ctx, path) }); err != nil {
		return unfiled.MetadataWarning()
	}
	return nil
}
