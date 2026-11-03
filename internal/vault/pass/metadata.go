package pass

import (
	"context"
	"os"
	"path/filepath"

	"lazypass/internal/vault"
	"lazypass/internal/vault/metadata"
)

func (p *Provider) DecorateMetadata(nodes []vault.Node) { metadata.Decorate(p.storeDir(), nodes) }

func (p *Provider) WriteMetadata(ctx context.Context, path vault.Path, kind vault.EntryType) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := metadata.Write(p.storeDir(), path, kind); err != nil {
		return err
	}
	return p.commitMetadata(ctx)
}

func (p *Provider) commitMetadata(ctx context.Context) error {
	if _, err := os.Stat(filepath.Join(p.storeDir(), ".git")); err != nil {
		return nil
	}
	if _, err := p.run(ctx, []string{"git", "add", "-A", "--", metadata.Directory}, nil); err != nil {
		return err
	}
	changes, err := p.run(ctx, []string{"git", "diff", "--cached", "--name-only", "--", metadata.Directory}, nil)
	if err != nil || len(changes) == 0 {
		return err
	}
	_, err = p.run(ctx, []string{"git", "commit", "-m", "Update vault type metadata", "--", metadata.Directory}, nil)
	return err
}
