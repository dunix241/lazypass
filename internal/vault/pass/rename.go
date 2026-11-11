package pass

import (
	"context"
	"lazypass/internal/vault"
)

func (p *Provider) Rename(ctx context.Context, node vault.Node, name string) error {
	if len(node.Path) == 0 {
		return vault.ErrInvalidPath
	}
	parent := node.Path[:len(node.Path)-1]
	if node.Kind == vault.FolderNode {
		return p.TransferFolder(ctx, node.Path, parent, name, true)
	}
	return p.Transfer(ctx, node.Path, parent, name, true)
}
