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
			s.storeMessage = "Items cannot be loaded. Type a destination instead."
		}
		return
	}
	needle := strings.ToLower(prefix)
	for _, node := range nodes {
		if strings.HasPrefix(strings.ToLower(node.Name), needle) {
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
	path, err := vault.ParsePath(parent)
	return path, prefix, err
}
func (s *StoreForm) insertStoreSuggestion() {
	if s.storeSelected < 0 || s.storeSelected >= len(s.storeNodes) {
		return
	}
	node := s.storeNodes[s.storeSelected]
	if node.Kind != vault.FolderNode {
		return
	}
	s.storePath = node.Path.String() + "/"
	s.refreshStoreSuggestions()
}

func (s *StoreForm) suggestionWindow(visible int) (start, end int) {
	if visible <= 0 || len(s.storeNodes) == 0 {
		return 0, 0
	}
	selected := max(0, min(s.storeSelected, len(s.storeNodes)-1))
	start = max(0, selected-visible+1)
	return start, min(start+visible, len(s.storeNodes))
}
