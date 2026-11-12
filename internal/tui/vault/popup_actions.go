package vault

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/tui/shared"
)

func popupAction(hint, label string, key tcell.Key, r rune) shared.Action {
	return shared.Action{Hint: hint, Label: label, Key: tcell.NewEventKey(key, r, 0)}
}

func popupFooterBounds(r shared.Rect) shared.Rect {
	return shared.Rect{X: r.X + 2, Y: r.Y + r.H - 2, W: max(0, r.W-4), H: 1}
}

func drawPopupActions(screen tcell.Screen, r shared.Rect, actions []shared.Action, color tcell.Color) {
	rows := (shared.Footer{Actions: actions}).Rows(max(0, r.W-4), shared.FooterOptions{MaxRows: 1})
	if len(rows) > 0 {
		shared.PrintPanelFooter(screen, r, rows[0], color)
	}
}

func popupActionAt(r shared.Rect, actions []shared.Action, event *tcell.EventMouse) (shared.Action, bool) {
	x, y := event.Position()
	return (shared.Footer{Actions: actions}).HitAction(popupFooterBounds(r), shared.FooterOptions{MaxRows: 1}, x, y)
}

func (s *View) conflictActions() []shared.Action {
	actions := []shared.Action{popupAction("r", "rename", tcell.KeyRune, 'r'), popupAction("s", "skip", tcell.KeyRune, 's')}
	if s.transfer.rename {
		actions = []shared.Action{popupAction("<CR>", "continue", tcell.KeyEnter, 0)}
	}
	return append(actions, popupAction("<esc>", "cancel", tcell.KeyEscape, 0))
}

func (s *StoreForm) qrActions() []shared.Action {
	back := popupAction("<esc>", "back", tcell.KeyEscape, 0)
	if s.qrImage != nil {
		if s.qrExport {
			return []shared.Action{popupAction("<CR>", "export", tcell.KeyEnter, 0), back}
		}
		return []shared.Action{popupAction("s", "export PNG", tcell.KeyRune, 's'), back}
	}
	if s.saving {
		return []shared.Action{popupAction("<esc>", "cancel", tcell.KeyEscape, 0)}
	}
	return []shared.Action{popupAction("<CR>", "file", tcell.KeyEnter, 0), popupAction("<ctrl-v>", "image", tcell.KeyCtrlV, 0), back}
}

func (s *StoreForm) bulkActions() []shared.Action {
	back := popupAction("<esc>", "cancel", tcell.KeyEscape, 0)
	if s.saving {
		return []shared.Action{back}
	}
	next := popupAction("<CR>", "next", tcell.KeyEnter, 0)
	switch s.bulk.phase {
	case "images":
		return []shared.Action{popupAction("<ctrl-v>", "clipboard", tcell.KeyCtrlV, 0), next, back}
	case "rename", "destination":
		back.Label = "back"
		return []shared.Action{next, back}
	case "results":
		next.Label = "finish"
		return []shared.Action{popupAction("r", "review", tcell.KeyRune, 'r'), popupAction("c", "reconcile", tcell.KeyRune, 'c'), next, back}
	default:
		return []shared.Action{popupAction("<space>", "select", tcell.KeyRune, ' '), popupAction("r", "rename", tcell.KeyRune, 'r'), next, back}
	}
}
