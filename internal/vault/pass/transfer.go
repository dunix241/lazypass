package pass

import (
	"bytes"
	"context"
	"lazypass/internal/vault"
	"lazypass/internal/vault/filetransfer"
	"lazypass/internal/vault/metadata"
	"os"
	"path/filepath"
)

func (p *Provider) transferPaths(ctx context.Context, source, destination vault.Path, name string) (*os.Root, vault.Path, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	target := append(append(vault.Path(nil), destination...), name)
	if err := requireEntryPath(source); err != nil {
		return nil, nil, err
	}
	if err := requireEntryPath(target); err != nil {
		return nil, nil, err
	}
	if err := p.safeEntry(source, true); err != nil {
		return nil, nil, err
	}
	parent, err := p.safeDirectory(destination)
	if err != nil {
		return nil, nil, err
	}
	if err := p.safeEntry(target, false); err != nil {
		return nil, nil, err
	}
	store, err := p.resolvedStoreDir()
	if err != nil {
		return nil, nil, err
	}
	if err := matchingRecipients(store, filepath.Join(store, filepath.Dir(source.String())), parent); err != nil {
		return nil, nil, err
	}
	r, err := os.OpenRoot(store)
	if err != nil {
		return nil, nil, vault.ErrUnavailable
	}
	if err := filetransfer.Check(r, source.String()+".gpg", target.String()+".gpg"); err != nil {
		r.Close()
		return nil, nil, err
	}
	return r, target, nil
}

// Relocating ciphertext unchanged requires the same effective recipient scope.
func matchingRecipients(store, source, destination string) error {
	read := func(parent string) ([]byte, error) {
		dir, err := recipientDirectory(store, parent)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(filepath.Join(dir, ".gpg-id"))
		if err != nil {
			return nil, vault.ErrUnavailable
		}
		return data, nil
	}
	a, err := read(source)
	if err != nil {
		return err
	}
	b, err := read(destination)
	if err != nil {
		return err
	}
	if !bytes.Equal(a, b) {
		return vault.ErrUnsupported
	}
	return nil
}

func (p *Provider) commitTransfer(ctx context.Context, paths []string, message string) error {
	if _, err := os.Stat(filepath.Join(p.storeDir(), ".git")); err != nil {
		return nil
	}
	if _, err := p.run(ctx, append([]string{"git", "add", "-A", "--"}, paths...), nil); err != nil {
		return vault.SavedWarning{}
	}
	if _, err := p.run(ctx, append([]string{"git", "commit", "-m", message, "--"}, paths...), nil); err != nil {
		return vault.SavedWarning{}
	}
	return nil
}

func (p *Provider) CheckTransfer(ctx context.Context, source, destination vault.Path, name string, move bool) error {
	r, _, err := p.transferPaths(ctx, source, destination, name)
	if r != nil {
		r.Close()
	}
	return err
}

func (p *Provider) Transfer(ctx context.Context, source, destination vault.Path, name string, move bool) error {
	r, target, err := p.transferPaths(ctx, source, destination, name)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := filetransfer.Execute(ctx, r, source.String()+".gpg", target.String()+".gpg", move); err != nil {
		return err
	}
	if err := metadata.Transfer(p.storeDir(), source, target, move); err != nil {
		return vault.SavedWarning{}
	}
	store, err := p.resolvedStoreDir()
	if err != nil {
		return vault.SavedWarning{}
	}
	if _, err := os.Stat(filepath.Join(store, ".git")); err == nil {
		paths := []string{target.String() + ".gpg"}
		if move {
			paths = append(paths, source.String()+".gpg")
		}
		for _, path := range []vault.Path{source, target} {
			if !move && path.String() == source.String() {
				continue
			}
			record := metadata.RecordPath(path)
			if _, err := os.Stat(filepath.Join(store, record)); err == nil {
				paths = append(paths, record)
			}
		}
		// Removed metadata must be staged as well when it was tracked.
		if move {
			tracked, err := p.run(ctx, []string{"git", "ls-files", "--", metadata.RecordPath(source)}, nil)
			if err != nil {
				return vault.SavedWarning{}
			}
			if len(tracked) > 0 {
				paths = append(paths, metadata.RecordPath(source))
			}
		}
		return p.commitTransfer(ctx, paths, "Transfer vault entry")
	}
	return nil
}
