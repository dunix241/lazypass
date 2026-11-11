package metadata

import (
	"lazypass/internal/vault"
	"path/filepath"
	"strings"
)

// TransferTree returns metadata paths affected by a recursive transfer for
// providers which track those records in Git.
func TransferTree(store string, source, target vault.Path, files []string, extension string, move bool) ([]string, error) {
	var paths []string
	for _, file := range files {
		if !strings.HasSuffix(file, extension) {
			continue
		}
		parts := strings.Split(filepath.ToSlash(strings.TrimSuffix(file, extension)), "/")
		from := append(append(vault.Path(nil), source...), parts...)
		to := append(append(vault.Path(nil), target...), parts...)
		nodes := []vault.Node{{Path: from, Kind: vault.EntryNode}}
		Decorate(store, nodes)
		if err := transfer(store, from, to, nodes[0], move); err != nil {
			return paths, err
		}
		if nodes[0].TypeKnown {
			paths = append(paths, RecordPath(to))
			if move {
				paths = append(paths, RecordPath(from))
			}
		}
	}
	return paths, nil
}

func Transfer(store string, source, target vault.Path, move bool) error {
	nodes := []vault.Node{{Path: source, Kind: vault.EntryNode}}
	Decorate(store, nodes)
	return transfer(store, source, target, nodes[0], move)
}

func transfer(store string, source, target vault.Path, node vault.Node, move bool) error {
	if node.TypeKnown {
		if err := Write(store, target, node.EntryType); err != nil {
			return err
		}
	}
	if move {
		return Remove(store, source, false)
	}
	return nil
}
