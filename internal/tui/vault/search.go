package vault

import (
	"context"
	"slices"
	"time"

	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

const searchTimeout = 30 * time.Second

// UI state is accessed only before dispatch or inside the dispatched callback.
func (s *View) loadSearchNodes() {
	s.CancelSearch()
	s.searchNodes = nil
	s.searchLoaded, s.searching = true, true
	ctx, cancel := context.WithCancel(context.Background())
	s.searchCancel = cancel
	generation := s.searchGeneration
	root := slices.Clone(s.vaultPath)
	initial := cloneNodes(s.vaultNodes)
	provider, dispatch := s.vault, s.deps.Dispatch
	work := func() {
		traversal, stop := context.WithTimeout(ctx, searchTimeout)
		results, partial := collectSearchNodes(traversal, provider, root, initial)
		stop()
		if ctx.Err() != nil {
			return
		}
		apply := func() {
			if generation != s.searchGeneration || !s.vaultFiltering {
				return
			}
			defer cancel()
			s.searchCancel = nil
			s.searchNodes = results
			s.searching = false
			s.vaultSelected = min(s.vaultSelected, max(0, len(s.filteredVaultNodes())-1))
			if partial {
				s.notify(shared.Warning, "Some folders could not be searched")
			}
		}
		if dispatch != nil {
			dispatch(ctx, apply)
		} else {
			apply()
		}
	}
	if dispatch != nil {
		go work()
	} else {
		work()
	}
}

func (s *View) CancelSearch() {
	if s.searchCancel != nil {
		s.searchCancel()
		s.searchCancel = nil
	}
	s.searchGeneration++
	s.searching = false
}

func cloneNodes(nodes []vault.Node) []vault.Node {
	result := slices.Clone(nodes)
	for i := range result {
		result[i].Path = slices.Clone(result[i].Path)
	}
	return result
}

func collectSearchNodes(ctx context.Context, provider service.Service, root vault.Path, initial []vault.Node) ([]vault.Node, bool) {
	queue := []vault.Path{root}
	seen := map[string]bool{root.String(): true}
	var results []vault.Node
	partial := false
	for head := 0; head < len(queue); head++ {
		if ctx.Err() != nil {
			return results, true
		}
		path := queue[head]
		nodes := initial
		if head > 0 {
			var err error
			nodes, err = provider.List(vault.WithoutPrompt(ctx), path)
			if err != nil {
				partial = true
				continue
			}
		}
		for _, node := range nodes {
			if ctx.Err() != nil {
				return results, true
			}
			if len(node.Path) != len(path)+1 || node.Path.Validate() != nil || !slices.Equal(node.Path[:len(path)], path) {
				continue
			}
			if node.Kind != vault.EntryNode && node.Kind != vault.FolderNode {
				continue
			}
			node.Path = slices.Clone(node.Path)
			results = append(results, node)
			if node.Kind == vault.FolderNode && !seen[node.Path.String()] {
				seen[node.Path.String()] = true
				queue = append(queue, node.Path)
			}
		}
	}
	return results, partial
}
