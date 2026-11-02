// Package service contains vault use cases shared by transports.
package service

import (
	"context"
	"errors"

	"lazypass/internal/vault"
)

var ErrClipboard = errors.New("clipboard unavailable")

type Service struct {
	Provider vault.Provider
	Copy     func(string) error
}

func (s Service) provider() (vault.Provider, error) {
	if s.Provider == nil {
		return nil, vault.ErrUnavailable
	}
	return s.Provider, nil
}

func (s Service) List(ctx context.Context, path vault.Path) ([]vault.Node, error) {
	p, err := s.provider()
	if err != nil {
		return nil, err
	}
	return p.List(ctx, path)
}

func (s Service) Show(ctx context.Context, path vault.Path) (vault.Entry, error) {
	p, err := s.provider()
	if err != nil {
		return vault.Entry{}, err
	}
	return p.Read(ctx, path)
}

func (s Service) Store(ctx context.Context, entry vault.Entry) error {
	p, err := s.provider()
	if err != nil {
		return err
	}
	return p.Write(ctx, entry)
}

// CheckNewPath prevents replacing an entry at the exact destination.
// Folders and entries may share a name in providers such as pass.
func (s Service) CheckNewPath(ctx context.Context, path vault.Path) error {
	if len(path) == 0 {
		return vault.ErrInvalidPath
	}
	if err := path.Validate(); err != nil {
		return err
	}
	nodes, err := s.List(ctx, path[:len(path)-1])
	if errors.Is(err, vault.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if node.Name == path[len(path)-1] && node.Kind == vault.EntryNode {
			return vault.ErrConflict
		}
	}
	return nil
}

func (s Service) Create(ctx context.Context, entry vault.Entry) error {
	p, err := s.provider()
	if err != nil {
		return err
	}
	if creator, ok := p.(vault.EntryCreator); ok {
		return creator.Create(ctx, entry)
	}
	if err := s.CheckNewPath(ctx, entry.Path); err != nil {
		return err
	}
	return s.Store(ctx, entry)
}

func (s Service) CreateFolder(ctx context.Context, path vault.Path) error {
	if len(path) == 0 {
		return vault.ErrInvalidPath
	}
	if err := path.Validate(); err != nil {
		return err
	}
	p, err := s.provider()
	if err != nil {
		return err
	}
	creator, ok := p.(vault.FolderCreator)
	if !ok {
		return vault.ErrUnsupported
	}
	return creator.CreateFolder(ctx, path)
}

func (s Service) Delete(ctx context.Context, node vault.Node) error {
	p, err := s.provider()
	if err != nil {
		return err
	}
	if !p.Capabilities().Delete {
		return vault.ErrUnsupported
	}
	if len(node.Path) == 0 {
		return vault.ErrInvalidPath
	}
	if err := node.Path.Validate(); err != nil {
		return err
	}
	if node.Kind == vault.FolderNode {
		deleter, ok := p.(vault.FolderDeleter)
		if !ok {
			return vault.ErrUnsupported
		}
		return deleter.DeleteFolder(ctx, node.Path)
	}
	if node.Kind != vault.EntryNode {
		return vault.ErrInvalidPath
	}
	return p.Delete(ctx, node.Path)
}

func (s Service) CopyPassword(ctx context.Context, path vault.Path) error {
	if s.Copy == nil {
		return ErrClipboard
	}
	entry, err := s.Show(ctx, path)
	if err != nil {
		return err
	}
	if err := s.Copy(entry.Password); err != nil {
		return ErrClipboard
	}
	return nil
}

func (s Service) Sync(ctx context.Context) (vault.SyncResult, error) {
	p, err := s.provider()
	if err != nil {
		return vault.SyncResult{}, err
	}
	return p.Sync(ctx)
}
