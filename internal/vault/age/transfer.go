package age

import (
	"context"
	"lazypass/internal/vault"
	"lazypass/internal/vault/filetransfer"
	"lazypass/internal/vault/metadata"
	"os"
)

func (p *Provider) transferPaths(ctx context.Context, source, destination vault.Path, name string) (*os.Root, string, string, vault.Path, error) {
	target := append(append(vault.Path(nil), destination...), name)
	from, err := checkedPath(source, true)
	if err != nil {
		return nil, "", "", nil, err
	}
	to, err := checkedPath(target, true)
	if err != nil {
		return nil, "", "", nil, err
	}
	r, err := p.open(ctx)
	if err != nil {
		return nil, "", "", nil, err
	}
	if _, err = readManifest(r); err == nil {
		err = checkParents(r, source[:len(source)-1], false)
	}
	if err == nil {
		err = checkParents(r, destination, false)
	}
	if err == nil {
		err = filetransfer.Check(r, from+".age", to+".age")
	}
	if err != nil {
		r.Close()
		return nil, "", "", nil, err
	}
	return r, from + ".age", to + ".age", target, nil
}

func (p *Provider) CheckTransfer(ctx context.Context, source, destination vault.Path, name string, move bool) error {
	r, _, _, _, err := p.transferPaths(ctx, source, destination, name)
	if r != nil {
		r.Close()
	}
	return err
}

func (p *Provider) Transfer(ctx context.Context, source, destination vault.Path, name string, move bool) error {
	r, from, to, target, err := p.transferPaths(ctx, source, destination, name)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := filetransfer.Execute(ctx, r, from, to, move); err != nil {
		return err
	}
	if err := metadata.Transfer(p.StoreDir, source, target, move); err != nil {
		return vault.SavedWarning{}
	}
	return nil
}
