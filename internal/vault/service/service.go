// Package service contains vault use cases shared by transports.
package service

import (
	"context"
	"fmt"

	"lazypass/internal/vault"
)

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

func (s Service) CopyPassword(ctx context.Context, path vault.Path) error {
	if s.Copy == nil {
		return fmt.Errorf("clipboard unavailable")
	}
	entry, err := s.Show(ctx, path)
	if err != nil {
		return err
	}
	return s.Copy(entry.Password)
}

func (s Service) Sync(ctx context.Context) (vault.SyncResult, error) {
	p, err := s.provider()
	if err != nil {
		return vault.SyncResult{}, err
	}
	return p.Sync(ctx)
}
