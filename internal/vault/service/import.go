package service

import (
	"context"
	"errors"

	"lazypass/internal/vault"
)

type ImportResult struct {
	Status  string
	Created bool
}

// ImportEntries never retries uncertain writes or rolls back confirmed creations.
func (s Service) ImportEntries(ctx context.Context, entries []vault.Entry, progress func(int)) []ImportResult {
	results := make([]ImportResult, len(entries))
	for i, entry := range entries {
		if ctx.Err() != nil {
			for j := i; j < len(results); j++ {
				results[j].Status = "Canceled; not attempted"
			}
			break
		}
		results[i] = s.importEntry(ctx, entry)
		if progress != nil {
			progress(i + 1)
		}
	}
	return results
}

func (s Service) importEntry(ctx context.Context, entry vault.Entry) ImportResult {
	if err := s.CheckNewPath(ctx, entry.Path); err != nil {
		if errors.Is(err, vault.ErrConflict) {
			return ImportResult{Status: "Name already exists; not imported"}
		}
		return ImportResult{Status: "Destination unavailable; not imported"}
	}
	if ctx.Err() != nil {
		return ImportResult{Status: "Canceled; not attempted"}
	}
	err := s.Create(ctx, entry)
	var warning vault.SavedWarning
	switch {
	case err == nil:
		return ImportResult{Status: "Imported", Created: true}
	case errors.As(err, &warning):
		return ImportResult{Status: "Imported; refresh warning", Created: true}
	case errors.Is(err, vault.ErrConflict):
		return ImportResult{Status: "Name already exists; not imported"}
	default:
		return ImportResult{Status: "Create failed; review destination before retry"}
	}
}
