// Package service contains vault use cases shared by transports.
package service

import (
	"context"
	"errors"
	"slices"

	"lazypass/internal/vault"
)

var ErrClipboard = errors.New("clipboard unavailable")
var ErrEmptyCopy = errors.New("this entry has no value to copy")
var ErrIdentityCopy = errors.New("identity entries have no default copy field")

type Service struct {
	Types    *TypeCache
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
	nodes, err := p.List(ctx, path)
	if err != nil {
		return nil, err
	}
	return s.Types.decorate(nodes), nil
}

func (s Service) Show(ctx context.Context, path vault.Path) (vault.Entry, error) {
	p, err := s.provider()
	if err != nil {
		return vault.Entry{}, err
	}
	entry, err := p.Read(ctx, path)
	if err == nil {
		s.Types.remember(entry)
	} else if errors.Is(err, vault.ErrNotFound) {
		s.Types.forget(vault.Node{Path: path, Kind: vault.EntryNode})
	}
	return entry, err
}

func (s Service) Store(ctx context.Context, entry vault.Entry) error {
	entry, err := vault.PrepareEntry(entry)
	if err != nil {
		return err
	}
	p, err := s.provider()
	if err != nil {
		return err
	}
	if err := p.Write(ctx, entry); err != nil {
		return err
	}
	s.Types.remember(entry)
	return nil
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
	if validator, ok := s.Provider.(vault.DestinationValidator); ok {
		if err := validator.ValidateDestination(ctx, path); err != nil {
			return err
		}
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
	entry.AuthenticatorChanged = entry.Authenticator != ""
	if !slices.Contains(s.SupportedEntryTypes(), entry.Type) {
		return vault.ErrUnsupported
	}
	entry, err := vault.PrepareEntry(entry)
	if err != nil {
		return err
	}
	p, err := s.provider()
	if err != nil {
		return err
	}
	if creator, ok := p.(vault.EntryCreator); ok {
		if err := creator.Create(ctx, entry); err != nil {
			return err
		}
		s.Types.remember(entry)
		return nil
	}
	return vault.ErrUnsupported
}

func (s Service) SupportedEntryTypes() []vault.EntryType {
	if provider, ok := s.Provider.(vault.EntryTypes); ok {
		return provider.SupportedEntryTypes()
	}
	return []vault.EntryType{vault.LoginEntry}
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
		if err := deleter.DeleteFolder(ctx, node.Path); err != nil {
			return err
		}
		s.Types.forget(node)
		return nil
	}
	if node.Kind != vault.EntryNode {
		return vault.ErrInvalidPath
	}
	if err := p.Delete(ctx, node.Path); err != nil {
		return err
	}
	s.Types.forget(node)
	return nil
}

func (s Service) RemoveFolder(ctx context.Context, path vault.Path) error {
	if len(path) == 0 || path.Validate() != nil {
		return vault.ErrInvalidPath
	}
	p, err := s.provider()
	if err != nil {
		return err
	}
	remover, ok := p.(vault.FolderRemover)
	if !ok {
		return vault.ErrUnsupported
	}
	return remover.RemoveFolder(ctx, path)
}

func (s Service) CopyPassword(ctx context.Context, path vault.Path, guards ...context.Context) error {
	if s.Copy == nil {
		return ErrClipboard
	}
	entry, err := s.Show(ctx, path)
	if err != nil {
		return err
	}
	value := entry.Password
	switch entry.Type {
	case vault.CardEntry:
		value = entry.Fields["number"]
	case vault.NoteEntry:
		value = entry.Notes
	case vault.SSHKeyEntry:
		value = entry.Fields["publicKey"]
	case vault.IdentityEntry:
		return ErrIdentityCopy
	case vault.LoginEntry:
	default:
		return vault.ErrUnsupported
	}
	if value == "" {
		return ErrEmptyCopy
	}
	return s.copyValue(ctx, value, guards...)
}

// Check the original cancellation sources as well as derived operation contexts:
// context.AfterFunc propagates cancellation asynchronously.
func (s Service) copyValue(ctx context.Context, value string, guards ...context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, guard := range guards {
		if err := guard.Err(); err != nil {
			return err
		}
	}
	if err := s.Copy(value); err != nil {
		return ErrClipboard
	}
	return nil
}

func (s Service) Sync(ctx context.Context) (vault.SyncResult, error) {
	p, err := s.provider()
	if err != nil {
		return vault.SyncResult{}, err
	}
	result, err := p.Sync(ctx)
	if err == nil {
		s.Types.clear()
	}
	return result, err
}
