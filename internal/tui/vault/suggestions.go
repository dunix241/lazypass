package vault

import (
	"context"
	"errors"
	"strings"

	"lazypass/internal/debug"
	"lazypass/internal/vault"
)

func (s *StoreForm) refreshStoreSuggestions() {
	if s.cancelSuggestions != nil {
		s.cancelSuggestions()
		s.cancelSuggestions = nil
	}
	s.suggestionPending = make(map[string]bool)
	s.suggestionGeneration++
	s.loadingEntries = false
	s.storeNodes, s.storeSelected, s.storeMessage = nil, 0, ""
	path, prefix, err := storeSuggestionQuery(s.storePath)
	if err != nil {
		return
	}
	nodes, cached := s.suggestions[path.String()]
	s.loadingEntries = !cached
	if !cached {
		key := path.String()
		if s.suggestionPending == nil {
			s.suggestionPending = make(map[string]bool)
		}
		if s.suggestionPending[key] {
			return
		}
		s.suggestionPending[key] = true
		generation := s.suggestionGeneration
		provider := s.deps.Service
		if s.cancelSuggestions != nil {
			s.cancelSuggestions()
		}
		request, cancel := context.WithCancel(context.Background())
		s.cancelSuggestions = cancel
		s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error {
			if request.Err() != nil {
				return request.Err()
			}
			ctx, stop := context.WithCancel(ctx)
			defer stop()
			detach := context.AfterFunc(request, stop)
			defer detach()
			var err error
			nodes, err = provider.List(ctx, path)
			return err
		}, func(err error) {
			defer cancel()
			if generation != s.suggestionGeneration || !s.open {
				return
			}
			delete(s.suggestionPending, key)
			s.loadingEntries = false
			if request.Err() != nil {
				return
			}
			if generation != s.suggestionGeneration || !s.open {
				return
			}
			delete(s.suggestionPending, key)
			s.loadingEntries = false
			if err != nil && !errors.Is(err, vault.ErrNotFound) {
				debug.Failure("vault suggestions", vaultFailureCause(err))
				s.storeMessage = "Items cannot be loaded. Type a destination instead."
				return
			}
			if s.suggestions == nil {
				s.suggestions = make(map[string][]vault.Node)
			}
			s.suggestions[path.String()] = nodes
			current, _, parseErr := storeSuggestionQuery(s.storePath)
			if parseErr == nil && current.String() == key {
				s.refreshStoreSuggestions()
			}
		})
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
	if s.locationOnly && len(s.location) >= 3 {
		return
	}
	if s.locationOnly {
		s.location = append(vault.Path(nil), node.Path...)
		s.entryName = ""
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
