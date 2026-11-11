package vault

import (
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
	mark, remove := "mark", "remove"
	if len(s.marks) > 0 {
		mark, remove = "unmark", "delete marked"
	}
	actions := []shared.Action{
		{Hint: "m", Label: mark, Priority: shared.Primary},
		{Hint: "M", Label: "mark range", Priority: shared.Primary},
		{Hint: "d", Label: remove, Priority: shared.Primary},
		{Hint: "D", Label: "recursive delete", Priority: shared.Secondary},
		{Hint: "a", Label: "add", Priority: shared.Primary},
		{Hint: "e", Label: "edit", Priority: shared.Primary},
		{Hint: "o", Label: "copy code", Priority: shared.Primary},
		{Hint: "r", Label: "refresh", Priority: shared.Secondary},
		{Hint: "h", Label: "back", Priority: shared.Essential},
		{Hint: "l", Label: "open/copy", Priority: shared.Essential},
		{Hint: "/", Label: "search", Priority: shared.Secondary},
		{Hint: "V", Label: "vault", Priority: shared.Secondary},
		{Hint: "t", Label: "themes", Priority: shared.Optional},
		{Hint: "v", Label: "view", Priority: shared.Essential},
		{Hint: "q", Label: "quit", Priority: shared.Essential},
	}
	return (shared.Footer{Actions: actions}).Rows(width-2, shared.FooterOptions{MaxRows: maxRows})
}
