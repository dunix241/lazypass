// Package unfiled plans filesystem container removal without discarding contents.
package unfiled

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"lazypass/internal/vault"
	"lazypass/internal/vault/metadata"
)

const Directory = "Unorganized"

type MoveFailure struct{ Err error }

func (e *MoveFailure) Error() string { return e.UserMessage() }
func (e *MoveFailure) Unwrap() error { return e.Err }
func (e *MoveFailure) UserMessage() string {
	return "Folder removal stopped; inspect the source and Unorganized before retrying"
}

func MetadataWarning() error {
	return vault.SavedWarning{Message: "Contents moved to Unorganized, but entry metadata needs repair"}
}

type Move struct {
	Source, Destination vault.Path
	Folder              bool
}

func Plan(ctx context.Context, root *os.Root, path vault.Path, extension string) ([]Move, error) {
	if len(path) == 0 || path.Validate() != nil || path[0] == Directory {
		return nil, vault.ErrInvalidPath
	}
	source := path.String()
	// Inspect every source component, not just the last directory.
	for i := range path {
		info, err := root.Lstat(path[:i+1].String())
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, vault.ErrInvalidPath
		}
	}
	if info, err := root.Lstat(Directory); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, vault.ErrInvalidPath
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	err := fs.WalkDir(root.FS(), source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return vault.ErrInvalidPath
		}
		if !entry.IsDir() && !entry.Type().IsRegular() {
			return vault.ErrUnsupported
		}
		if !entry.IsDir() && !strings.HasSuffix(name, extension) && (extension != ".gpg" || filepath.Base(name) != ".gpg-id") {
			return vault.ErrUnsupported
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(root.FS(), source)
	if err != nil {
		return nil, err
	}
	var moves []Move
	for _, entry := range entries {
		if entry.Name() == ".gpg-id" {
			continue
		}
		dest := Directory + "/" + entry.Name()
		if _, err := root.Lstat(dest); err == nil {
			return nil, vault.ErrConflict
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		name := entry.Name()
		if !entry.IsDir() {
			name = strings.TrimSuffix(name, extension)
		}
		from := append(append(vault.Path(nil), path...), name)
		to := vault.Path{Directory, name}
		if from.Validate() != nil || to.Validate() != nil {
			return nil, vault.ErrInvalidPath
		}
		moves = append(moves, Move{from, to, entry.IsDir()})
	}
	return moves, nil
}

func RemapMetadata(store string, moves []Move, list func(vault.Path) ([]vault.Node, error)) error {
	for _, move := range moves {
		var nodes []vault.Node
		if move.Folder {
			var visit func(vault.Path) error
			visit = func(path vault.Path) error {
				children, err := list(path)
				if err != nil {
					return err
				}
				for _, node := range children {
					if node.Kind == vault.FolderNode {
						if err := visit(node.Path); err != nil {
							return err
						}
					} else {
						nodes = append(nodes, node)
					}
				}
				return nil
			}
			if err := visit(move.Destination); err != nil {
				return err
			}
		} else {
			nodes = []vault.Node{{Path: move.Destination, Kind: vault.EntryNode}}
		}
		for _, node := range nodes {
			old := append(append(vault.Path(nil), move.Source...), node.Path[len(move.Destination):]...)
			probe := []vault.Node{{Path: old, Kind: vault.EntryNode}}
			metadata.Decorate(store, probe)
			if probe[0].TypeKnown {
				if err := metadata.Write(store, node.Path, probe[0].EntryType); err != nil {
					return err
				}
			}
		}
		if err := metadata.Remove(store, move.Source, move.Folder); err != nil {
			return err
		}
	}
	return nil
}
