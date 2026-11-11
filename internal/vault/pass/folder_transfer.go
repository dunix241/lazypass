package pass

import (
	"context"
	"lazypass/internal/vault"
	"lazypass/internal/vault/filetransfer"
	"lazypass/internal/vault/metadata"
	"os"
	"path/filepath"
)

func (p *Provider) folderTransferPlan(ctx context.Context, source, destination vault.Path, name string) (*os.Root, vault.Path, filetransfer.Tree, error) {
	target := append(append(vault.Path(nil), destination...), name)
	var tree filetransfer.Tree
	if err := requireEntryPath(source); err != nil {
		return nil, nil, tree, err
	}
	if err := requireEntryPath(target); err != nil {
		return nil, nil, tree, err
	}
	from, err := p.safeDirectory(source)
	if err != nil {
		return nil, nil, tree, err
	}
	parent, err := p.safeDirectory(destination)
	if err != nil {
		return nil, nil, tree, err
	}
	store, err := p.resolvedStoreDir()
	if err != nil {
		return nil, nil, tree, err
	}
	// An explicit recipient scope travels with the folder. Otherwise inherited
	// recipients must match before ciphertext can be relocated unchanged.
	if _, err := os.Lstat(filepath.Join(from, ".gpg-id")); os.IsNotExist(err) {
		if err := matchingRecipients(store, from, parent); err != nil {
			return nil, nil, tree, err
		}
	} else if err != nil {
		return nil, nil, tree, vault.ErrUnavailable
	}
	r, err := os.OpenRoot(store)
	if err != nil {
		return nil, nil, tree, vault.ErrUnavailable
	}
	tree, err = filetransfer.Inspect(ctx, r, source.String(), target.String(), ".gpg")
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
	paths := []string{target.String()}
	if move {
		paths = append(paths, source.String())
	}
	metaPaths, err := metadata.TransferTree(p.storeDir(), source, target, tree.Files, ".gpg", move)
	if err != nil {
		return vault.SavedWarning{}
	}
	paths = append(paths, metaPaths...)
	return p.commitTransfer(ctx, paths, "Transfer vault folder")
}
