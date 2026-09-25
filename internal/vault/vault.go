// Package vault defines the provider-neutral password vault contract.
package vault

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var (
	ErrNotFound      = errors.New("vault entry not found")
	ErrLocked        = errors.New("vault is locked")
	ErrConflict      = errors.New("vault sync conflict")
	ErrUnavailable   = errors.New("vault is unavailable")
	ErrUninitialized = errors.New("vault is not initialized")
	ErrInvalidPath   = errors.New("invalid vault path")
)

type Path []string

func (p Path) Validate() error {
	for _, part := range p {
		if part == "" || part == "." || part == ".." || part == ".gpg-id" || part == ".git" || strings.ContainsAny(part, `/\\`) || strings.HasSuffix(part, ".gpg") {
			return fmt.Errorf("%w: invalid component", ErrInvalidPath)
		}
		for _, r := range part {
			if unicode.IsControl(r) {
				return fmt.Errorf("%w: control character", ErrInvalidPath)
			}
		}
	}
	return nil
}

func (p Path) String() string { return strings.Join(p, "/") }

type NodeKind uint8

const (
	FolderNode NodeKind = iota
	EntryNode
)

type Node struct {
	Path Path
	Name string
	Kind NodeKind
}

type Entry struct {
	Path     Path
	Password string
	Username string
	URL      string
	Notes    string
	Fields   map[string]string
}

type Capabilities struct {
	Sync, Delete bool
}

type SyncResult struct {
	Pulled bool
	Pushed bool
}

type Provider interface {
	Name() string
	Capabilities() Capabilities
	List(context.Context, Path) ([]Node, error)
	Read(context.Context, Path) (Entry, error)
	Write(context.Context, Entry) error
	Delete(context.Context, Path) error
	Sync(context.Context) (SyncResult, error)
}
