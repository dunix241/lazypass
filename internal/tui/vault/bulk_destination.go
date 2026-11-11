package vault

import (
	"context"
	"errors"

	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

func (s *StoreForm) loadBulkDestination(path vault.Path) {
	b := s.bulk
	path = append(vault.Path(nil), path...)
	var nodes []vault.Node
	provider := s.deps.Service
	s.run(func(ctx context.Context) error {
		var err error
		nodes, err = provider.List(ctx, path)
		return err
	}, func(err error) {
		if err != nil {
			s.notify(shared.Error, "Cannot open destination")
			return
		}
		b.destination, b.folderIndex = path, 0
		b.destinationViewport = shared.ListViewport{}
		b.text = path.String()
		b.folders = nil
		for _, node := range nodes {
			if node.Kind == vault.FolderNode {
				b.folders = append(b.folders, node)
			}
		}
	})
}

func (s *StoreForm) reconcileBulk() {
	b := s.bulk
	rows := append([]bulkRow(nil), b.rows...)
	provider := s.deps.Service
	s.run(func(ctx context.Context) error {
		if provider.Provider.Capabilities().Sync {
			if _, err := provider.Sync(ctx); err != nil {
				return err
			}
		}
		for i, row := range rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			if !row.uncertain {
				continue
			}
			entry, err := provider.Show(ctx, row.attemptedPath)
			switch {
			case errors.Is(err, vault.ErrNotFound):
				rows[i].uncertain = false
				rows[i].status = "Not found after refresh; available for retry"
			case err == nil && entry.Authenticator == row.record.URI:
				rows[i].created = true
				rows[i].uncertain = false
				rows[i].status = "Matching authenticator found; not retried"
			}
		}
		return nil
	}, func(err error) {
		if err != nil {
			s.notify(shared.Error, "Cannot reconcile import results")
			return
		}
		b.rows = rows
		for _, row := range rows {
			b.changed = b.changed || row.created
		}
	})
}

func (s *StoreForm) handleBulkDestination(event *tcell.EventKey) {
	b := s.bulk
	switch event.Key() {
	case tcell.KeyUp:
		b.folderIndex = max(0, b.folderIndex-1)
	case tcell.KeyDown:
		b.folderIndex = min(len(b.folders), b.folderIndex+1)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		s.bulkDestinationParent()
	case tcell.KeyEnter:
		if b.folderIndex == 0 {
			s.checkBulkConflicts()
		} else {
			s.loadBulkDestination(b.folders[b.folderIndex-1].Path)
		}
	case tcell.KeyRune:
		switch event.Rune() {
		case 'j':
			b.folderIndex = min(len(b.folders), b.folderIndex+1)
		case 'k':
			b.folderIndex = max(0, b.folderIndex-1)
		case 'h':
			s.bulkDestinationParent()
		case 'l':
			if b.folderIndex > 0 {
				s.loadBulkDestination(b.folders[b.folderIndex-1].Path)
			}
		}
	}
}

func (s *StoreForm) bulkDestinationParent() {
	path := s.bulk.destination
	if len(path) > 0 {
		s.loadBulkDestination(path[:len(path)-1])
	}
}

func (s *StoreForm) checkBulkConflicts() {
	b := s.bulk
	entries := make([]vault.Path, len(b.rows))
	seen := make(map[string]bool)
	for i, row := range b.rows {
		if !row.importable() {
			continue
		}
		path, err := row.destinationPath(b.destination)
		if err != nil {
			b.phase = "preview"
			s.notify(shared.Warning, "Rename invalid entry names before importing")
			return
		}
		if seen[path.String()] {
			b.rows[i].status = "Duplicate selected name; rename or skip"
			b.phase = "preview"
			return
		}
		seen[path.String()] = true
		entries[i] = path
	}
	provider := s.deps.Service
	conflicts := make([]bool, len(entries))
	s.run(func(ctx context.Context) error {
		for i, path := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			if path == nil {
				continue
			}
			if err := provider.CheckNewPath(ctx, path); err != nil {
				conflicts[i] = true
			}
		}
		return ctx.Err()
	}, func(err error) {
		if err != nil {
			s.notify(shared.Error, "Destination check failed")
			return
		}
		blocked := false
		for i, conflict := range conflicts {
			if conflict {
				b.rows[i].status = "Name conflict or unavailable destination; rename or skip"
				blocked = true
			} else if entries[i] != nil {
				b.rows[i].status = ""
			}
		}
		if blocked {
			b.phase = "preview"
			return
		}
		b.text = b.destination.String()
		s.executeBulk()
	})
}
