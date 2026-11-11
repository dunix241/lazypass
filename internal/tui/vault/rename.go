package vault

import (
	"context"
	"errors"
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/tui/input"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"slices"
)

func (s *View) openRename() {
	if s.deleting || s.transfer.busy {
		return
	}
	node, ok := s.selectedVaultNode()
	if !ok || node.Protected || s.isVaultParentNode(node) {
		return
	}
	if _, ok := s.vault.Provider.(vault.Renamer); !ok {
		s.notify(shared.Warning, "Rename unavailable")
		return
	}
	node.Path = slices.Clone(node.Path)
	s.renameTarget = &node
	s.transfer.editor.Sync(node.Name)
}

func (s *View) handleRenameKey(event *tcell.EventKey) {
	if event.Key() == tcell.KeyEscape {
		s.renameTarget = nil
		return
	}
	if event.Key() != tcell.KeyEnter {
		_, _ = s.transfer.editor.HandleKey(event, input.Options{MaxBytes: 255})
		return
	}
	node := *s.renameTarget
	name := s.transfer.editor.Value()
	if name == "" || (vault.Path{name}).Validate() != nil {
		return
	}
	if name == node.Name {
		s.renameTarget = nil
		return
	}
	s.renameTarget = nil
	s.transfer.busy = true
	provider := s.vault.Provider.(vault.Renamer)
	request, cancel := context.WithCancel(context.Background())
	s.transfer.cancel = cancel
	s.progress("Renaming…")
	s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error {
		if err := request.Err(); err != nil {
			return err
		}
		ctx, stop := operationContext(ctx, request)
		defer stop()
		return provider.Rename(ctx, node, name)
	}, func(err error) {
		canceled := request.Err() != nil
		cancel()
		s.transfer.cancel = nil
		s.transfer.busy = false
		if canceled {
			s.transfer.blocked = true
			return
		}
		var saved vault.SavedWarning
		if err != nil && !errors.As(err, &saved) {
			s.Refresh()
			s.notify(shared.Error, storeErrorMessage(err))
			return
		}
		// Local paths change on rename; remote IDs remain stable. Drop stale
		// staged references rather than pasting a different entry at the old path.
		s.transfer.nodes = nil
		s.ClearMarks()
		s.Reconcile("Renamed in vault")
		if err != nil {
			s.notify(shared.Warning, storeErrorMessage(err))
		}
	})
}

func (s *View) drawRename(screen tcell.Screen, frame shared.Frame) {
	screen.HideCursor()
	r := shared.CenteredPanel(frame.Viewport, min(64, max(0, frame.Viewport.W-2)), 7)
	shared.Panel{Bounds: r, Title: "Rename", Border: frame.Palette.Focus, Background: frame.Palette.Surface}.Draw(screen, func(content shared.Rect) {
		shared.PrintAt(screen, content.X, content.Y, "Name", frame.Palette.Text)
		s.transfer.editor.Draw(screen, content.X, content.Y+2, content.W, 1, tcell.StyleDefault.Foreground(frame.Palette.Text).Background(frame.Palette.Surface), true, input.Options{MaxBytes: 255})
		shared.PrintAt(screen, content.X, content.Y+content.H-1, "<CR> rename  •  <esc> cancel", frame.Palette.Muted)
	})
}
