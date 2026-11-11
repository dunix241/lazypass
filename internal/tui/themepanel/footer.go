package themepanel

import (
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/tui/shared"
)

func (s *Panel) footerActions() []shared.Action {
	var actions []shared.Action
	switch s.mode {
	case themePicker:
		if !s.filtering {
			actions = []shared.Action{{Hint: "a", Label: "add"}, {Hint: "e", Label: "edit"}, {Hint: "d", Label: "delete"}, {Hint: "/", Label: "filter"}}
		}
	case themeName:
		actions = []shared.Action{{Hint: "<enter>", Label: "create", Key: tcell.NewEventKey(tcell.KeyEnter, 0, 0)}}
	case themeEditor:
		actions = []shared.Action{{Hint: "<enter>", Label: "validate", Key: tcell.NewEventKey(tcell.KeyEnter, 0, 0)}, {Hint: "s", Label: "save"}}
	case themeDeleteConfirm:
		actions = []shared.Action{{Hint: "y", Label: "yes"}, {Hint: "<esc>", Label: "cancel", Key: tcell.NewEventKey(tcell.KeyEscape, 0, 0)}}
	}
	for i := range actions {
		if actions[i].Hint == "/" {
			actions[i].Priority = shared.Essential
		}
		if actions[i].Key == nil {
			actions[i].Key = tcell.NewEventKey(tcell.KeyRune, rune(actions[i].Hint[0]), 0)
		}
	}
	return actions
}

func themePickerHelp(width int) string {
	rows := (shared.Footer{Actions: (&Panel{}).footerActions()}).Rows(width, shared.FooterOptions{MaxRows: 1})
	if len(rows) == 0 {
		return ""
	}
	return rows[0]
}
