package vault

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/tui/shared"
	"strings"
)

func (s *View) Footer(width int) string {
	return strings.Join(s.FooterRows(width, 1), "")
}

func (s *View) FooterRows(width, maxRows int) []string {
	if s.IsConfirmingDelete() {
		if s.deleteDialog != nil && s.deleteDialog.Options.RequireText != "" {
			return []string{shared.FitFooter(width, "<CR> confirm  •  <esc> cancel", "<esc> cancel")}
		}
		return []string{shared.FitFooter(width, "y yes  •  n no", "y/n")}
	}
	return s.FooterActions().Rows(width-2, shared.FooterOptions{MaxRows: maxRows})
}

func (s *View) FooterActions() shared.Footer {
	if s.renameTarget != nil {
		return shared.Footer{Actions: []shared.Action{
			{Hint: "CR", Label: "rename", Key: tcell.NewEventKey(tcell.KeyEnter, 0, 0), Priority: shared.Essential},
			{Hint: "esc", Label: "cancel", Key: tcell.NewEventKey(tcell.KeyEscape, 0, 0), Priority: shared.Essential},
		}}
	}
	if s.IsTransferActive() {
		return shared.Footer{Actions: []shared.Action{{Hint: "esc", Label: "cancel paste", Key: tcell.NewEventKey(tcell.KeyEscape, 0, 0), Priority: shared.Essential}}}
	}
	mark, remove := "mark", "remove"
	if len(s.marks) > 0 {
		mark, remove = "unmark", "delete marked"
	}
	actions := make([]shared.Action, 0, 21)
	if len(s.transfer.nodes) > 0 {
		actions = append(actions, shared.Action{Hint: "p", Label: "paste", Priority: shared.Primary})
	}
	actions = append(actions, []shared.Action{
		{Hint: "y", Label: "yank", Priority: shared.Primary},
		{Hint: "x", Label: "cut", Priority: shared.Primary},
		{Hint: "c", Label: "copy", Priority: shared.Primary},
		{Hint: "m", Label: mark, Priority: shared.Primary},
		{Hint: "M", Label: "mark range", Priority: shared.Primary},
		{Hint: "d", Label: remove, Priority: shared.Primary},
		{Hint: "D", Label: "recursive delete", Priority: shared.Secondary},
		{Hint: "a", Label: "add", Priority: shared.Primary},
		{Hint: "e", Label: "edit", Priority: shared.Primary},
		{Hint: "C", Label: "copy code", Priority: shared.Primary},
		{Hint: "r", Label: "rename", Priority: shared.Primary},
		{Hint: "R", Label: "refresh", Priority: shared.Secondary},
		{Hint: "h", Label: "back", Priority: shared.Essential},
		{Hint: "l", Label: "open/copy", Priority: shared.Essential},
		{Hint: "/", Label: "search", Priority: shared.Secondary},
		{Hint: "V", Label: "vault", Priority: shared.Secondary},
		{Hint: "t", Label: "themes", Priority: shared.Optional},
		{Hint: "v", Label: "view", Priority: shared.Essential},
		{Hint: "q", Label: "quit", Priority: shared.Essential},
	}...)
	for i := range actions {
		actions[i].Key = tcell.NewEventKey(tcell.KeyRune, rune(actions[i].Hint[0]), 0)
	}
	return shared.Footer{Actions: actions}
}
