package service

import (
	"context"
	"slices"

	"lazypass/internal/vault"
)

type RebuildResult struct{ Indexed, Failed int }

func (s Service) RebuildMetadata(ctx context.Context) (RebuildResult, error) {
	var result RebuildResult
	writer, ok := s.Provider.(vault.MetadataWriter)
	if !ok {
		return result, vault.ErrUnsupported
	}
	queue := []vault.Path{nil}
	seen := map[string]bool{"": true}
	for head := 0; head < len(queue); head++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		nodes, err := s.List(ctx, queue[head])
		if err != nil {
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			result.Failed++
			continue
		}
		for _, node := range nodes {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			parent := queue[head]
			if len(node.Path) != len(parent)+1 || node.Path.Validate() != nil || !slices.Equal(node.Path[:len(parent)], parent) {
				continue
			}
			if node.Kind == vault.FolderNode {
				if !seen[node.Path.String()] {
					seen[node.Path.String()] = true
					queue = append(queue, slices.Clone(node.Path))
				}
				continue
			}
			if node.Kind != vault.EntryNode {
				continue
			}
			entry, err := s.Provider.Read(ctx, node.Path)
			if err == nil {
				err = writer.WriteMetadata(ctx, node.Path, entry.Type)
			}
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			if err != nil {
				result.Failed++
			} else {
				result.Indexed++
			}
		}
	}
	return result, nil
}
