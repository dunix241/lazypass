package vault

import (
	"context"
	"errors"
	"unicode/utf8"

	"lazypass/internal/debug"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

func (s *View) openDelete() {
	if s.deleting {
		return
	}
	node, ok := s.selectedVaultNode()
	if !ok || s.isVaultParentNode(node) || len(node.Path) == 0 {
		return
	}
	if s.vault.Provider == nil || !s.vault.Provider.Capabilities().Delete {
		s.notify(shared.Warning, "Deletion unavailable")
		return
	}
	node.Path = append(vault.Path(nil), node.Path...)
	s.deleteTarget = &node
}

func (s *View) handleDeleteKey(event *tcell.EventKey) {
	if event.Key() == tcell.KeyEscape {
		s.deleteTarget = nil
		return
	}
	if event.Key() != tcell.KeyRune {
		return
	}
	switch event.Rune() {
	case 'n', 'N', 'q':
		s.deleteTarget = nil
	case 'y', 'Y':
		node := *s.deleteTarget
		s.deleteTarget = nil
		s.deleting = true
		s.notify(shared.Info, "Deleting…")
		s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error { return s.vault.Delete(ctx, node) }, func(err error) {
			s.deleting = false
			if err != nil {
				debug.Failure("vault delete", vaultFailureCause(err))
				message := "Delete failed"
				if errors.Is(err, vault.ErrUnsupported) {
					message = "Deletion unavailable"
				}
				s.notify(shared.Error, message)
				return
			}
			s.Refresh()
			s.notify(shared.Success, "Deleted from vault")
		})
	}
}

func (s *View) drawDelete(screen tcell.Screen, frame shared.Frame) {
	if s.deleteTarget == nil {
		return
	}
	v := frame.Viewport
	w := min(60, v.W-2)
	h := 7
	if w < 20 || v.H < h {
		return
	}
	r := shared.Rect{X: v.X + (v.W-w)/2, Y: v.Y + (v.H-h)/2, W: w, H: h}
	colors := frame.Palette
	shared.DrawBox(screen, r, colors.Warning, colors.Surface)
	title := " Delete entry "
	if s.deleteTarget.Kind == vault.FolderNode {
		title = " Delete folder "
	}
	shared.PrintAt(screen, r.X+2, r.Y, shared.Truncate(title, w-4), colors.Warning)
	shared.PrintAt(screen, r.X+2, r.Y+2, shared.Truncate(s.deleteTarget.Path.String(), w-4), colors.Text)
	shared.PrintPanelFooter(screen, r, "y yes  •  n no", colors.Muted)
	const prefix = "y yes  •  "
	const choices = prefix + "n no"
	choiceX := r.X + max(1, (r.W-utf8.RuneCountInString(choices))/2)
	shared.PrintAt(screen, choiceX+utf8.RuneCountInString(prefix), r.Y+r.H-2, "n no", colors.Text)
}
