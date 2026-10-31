package vault

import (
	"context"
	"errors"
	"strings"

	"lazypass/internal/debug"
	"lazypass/internal/vault"
)

func (s *StoreForm) refreshStoreSuggestions() {
	s.storeNodes, s.storeSelected, s.storeMessage = nil, 0, ""
	path, prefix, err := storeSuggestionQuery(s.storePath)
	if err != nil {
		return
	}
	nodes, err := s.deps.Service.List(context.Background(), path)
	if err != nil {
		if !errors.Is(err, vault.ErrNotFound) {
			debug.Failure("vault suggestions", vaultFailureCause(err))
			s.storeMessage = "Folders cannot be loaded. Type a destination instead."
		}
		return
	}
	for _, node := range nodes {
		if node.Kind == vault.FolderNode && strings.HasPrefix(strings.ToLower(node.Name), strings.ToLower(prefix)) {
			s.storeNodes = append(s.storeNodes, node)
		}
	}
}
func storeSuggestionQuery(destination string) (vault.Path, string, error) {
	if destination == "" {
		return nil, "", nil
	}
	separator := strings.LastIndex(destination, "/")
	if separator < 0 {
		return nil, destination, nil
	}
	parent, prefix := destination[:separator], destination[separator+1:]
	if parent == "" {
		return nil, prefix, nil
	}
	path, err := vaultPath(parent)
	return path, prefix, err
}
func (s *StoreForm) insertStoreSuggestion() {
	if len(s.storeNodes) == 0 {
		return
	}
	s.storePath = s.storeNodes[s.storeSelected].Path.String() + "/"
	s.refreshStoreSuggestions()
}
func vaultPath(text string) (vault.Path, error) {
	path := vault.Path(strings.Split(text, "/"))
	if err := path.Validate(); err != nil {
		return nil, err
	}
	return path, nil
}
