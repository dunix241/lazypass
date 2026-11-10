package bitwarden

import (
	"slices"
	"sort"

	"lazypass/internal/vault"
)

const noFolderID = "no-folder"

func (s snapshot) folderID(reference string) (string, error) {
	if reference == noFolderID {
		return "", nil
	}
	for _, folder := range s.folders {
		if folder.ID == reference {
			return reference, nil
		}
	}
	return "", vault.ErrNotFound
}

func (s snapshot) checkFolderDestination(path vault.Path) error {
	if len(path) == 0 {
		return vault.ErrInvalidPath
	}
	for _, entries := range [][]item{s.items, s.excluded} {
		for _, entry := range entries {
			candidate, ok := s.path(entry)
			if ok && slices.Equal(candidate, path) {
				return vault.ErrConflict
			}
		}
	}
	return nil
}

func (s snapshot) listFolders(path vault.Path) ([]vault.Node, error) {
	var nodes []vault.Node
	switch len(path) {
	case 1:
		nodes = append(nodes, vault.Node{Name: "No Folder", Path: vault.Path{"Folders", noFolderID}, Kind: vault.FolderNode, Protected: true})
		for _, folder := range s.folders {
			nodes = append(nodes, vault.Node{Name: folder.Name, Path: vault.Path{"Folders", folder.ID}, Kind: vault.FolderNode})
		}
	case 2:
		folderID, err := s.folderID(path[1])
		if err != nil {
			return nil, err
		}
		for _, entries := range [][]item{s.items, s.excluded} {
			for _, entry := range entries {
				if entry.FolderID != folderID || entry.Type < 1 || entry.Type > 5 {
					continue
				}
				nodes = append(nodes, vault.Node{Name: entry.Name, Path: vault.Path{"Folders", path[1], entry.ID}, Kind: vault.EntryNode, EntryType: entryType(entry.Type), TypeKnown: true, Shared: entry.OrganizationID != ""})
			}
		}
	default:
		return nil, vault.ErrNotFound
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Name == nodes[j].Name {
			return nodes[i].Path.String() < nodes[j].Path.String()
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

func (s snapshot) resolveFolderItem(path vault.Path) (item, error) {
	if len(path) != 3 {
		return item{}, vault.ErrNotFound
	}

	folderID, err := s.folderID(path[1])
	if err != nil {
		return item{}, err
	}
	for _, entries := range [][]item{s.items, s.excluded} {
		for _, entry := range entries {
			if entry.ID == path[2] && entry.FolderID == folderID && entry.Type >= 1 && entry.Type <= 5 {
				return entry, nil
			}
		}
	}
	return item{}, vault.ErrNotFound
}

func (s snapshot) folderNamePath(path vault.Path) (vault.Path, error) {
	if len(path) < 2 || path[0] != "Folders" {
		return path, nil
	}
	if path[1] == noFolderID {
		return append(vault.Path(nil), path[2:]...), nil
	}
	for _, folder := range s.folders {
		if folder.ID == path[1] {
			parent, err := vault.ParsePath(folder.Name)
			if err != nil {
				return nil, err
			}
			return append(parent, path[2:]...), nil
		}
	}
	// New typed destinations beneath Folders use names, not IDs.
	return append(vault.Path(nil), path[1:]...), nil
}
