package vault

import (
	"context"

	"lazypass/internal/vault"
)

// Refresh only public type records: no decryption, sync, or provider listing.
func (s *View) RefreshMetadata(ctx context.Context) {
	reader, ok := s.vault.Provider.(vault.MetadataReader)
	if !ok || s.loading || s.metadataLoading || s.deps.Dispatch == nil || ctx.Err() != nil || len(s.vaultNodes)+len(s.searchNodes) == 0 {
		return
	}
	s.metadataLoading = true
	nodes, search := cloneNodes(s.vaultNodes), cloneNodes(s.searchNodes)
	generation, searchGeneration := s.loadGeneration, s.searchGeneration
	go func() {
		if ctx.Err() != nil {
			return
		}
		reader.DecorateMetadata(nodes)
		reader.DecorateMetadata(search)
		if ctx.Err() != nil {
			return
		}
		s.deps.Dispatch(ctx, func() {
			s.metadataLoading = false
			if generation != s.loadGeneration {
				return
			}
			applyNodeTypes(s.vaultNodes, nodes)
			if searchGeneration == s.searchGeneration && len(search) == len(s.searchNodes) {
				applyNodeTypes(s.searchNodes, search)
			}
		})
	}()
}

func applyNodeTypes(destination, source []vault.Node) {
	for i, node := range source {
		if i >= len(destination) {
			break
		}
		if node.Kind != vault.EntryNode || destination[i].Kind != node.Kind || node.Path.String() != destination[i].Path.String() {
			continue
		}
		destination[i].EntryType = node.EntryType
		destination[i].TypeKnown = node.TypeKnown
	}
}
