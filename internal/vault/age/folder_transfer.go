package age

import (
	"context"
	"lazypass/internal/vault"
	"lazypass/internal/vault/filetransfer"
	"lazypass/internal/vault/metadata"
	"os"
)

func (p *Provider) folderTransferPlan(ctx context.Context, source, destination vault.Path, name string) (*os.Root, vault.Path, filetransfer.Tree, error) {
	target := append(append(vault.Path(nil), destination...), name)
	var tree filetransfer.Tree
	from, err := checkedPath(source, true)
	if err != nil {
		return nil, nil, tree, err
	}
	to, err := checkedPath(target, true)
	if err != nil {
		return nil, nil, tree, err
	}
	r, err := p.open(ctx)
	if err != nil {
		return nil, nil, tree, err
	}
	if _, err = readManifest(r); err == nil {
		err = checkParents(r, source, false)
	}
	if err == nil {
		err = checkParents(r, destination, false)
	}
	if err == nil {
		tree, err = filetransfer.Inspect(ctx, r, from, to, ".age")
	}
	if err != nil {
		r.Close()
		return nil, nil, tree, err
	}
	return r, target, tree, nil
}

func (p *Provider) CheckFolderTransfer(ctx context.Context, source, destination vault.Path, name string, move bool) error {
	r, _, _, err := p.folderTransferPlan(ctx, source, destination, name)
	if r != nil {
		r.Close()
	}
	return err
}

func (p *Provider) TransferFolder(ctx context.Context, source, destination vault.Path, name string, move bool) error {
	r, target, tree, err := p.folderTransferPlan(ctx, source, destination, name)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := filetransfer.ExecuteTree(ctx, r, source.String(), target.String(), tree, move); err != nil {
		return err
	}
	if _, err := metadata.TransferTree(p.StoreDir, source, target, tree.Files, ".age", move); err != nil {
		return vault.SavedWarning{}
	}
	return nil
}
