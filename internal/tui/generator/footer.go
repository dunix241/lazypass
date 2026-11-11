package generator

import (
	"strings"

	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

func (s *View) FooterActions() shared.Footer {
	actions := []shared.Action{}
	if s.selected == focusLength {
		actions = append(actions, shared.Action{
			Hint: "←/→", Label: "length", Priority: shared.Essential,
			OnClick: func() { s.selected = focusLength },
		})
	} else if s.selected >= focusUpper && s.selected <= focusAmbiguous {
		actions = append(actions, shared.Action{Hint: "<space>", Label: "toggle", Priority: shared.Essential, Key: tcell.NewEventKey(tcell.KeyRune, ' ', 0)})
	}
	actions = append(actions,
		shared.Action{Hint: "<tab>", Label: "focus", Priority: shared.Secondary, Key: tcell.NewEventKey(tcell.KeyTAB, 0, 0)},
		shared.Action{Hint: "a", Label: "add", Priority: shared.Primary},
		shared.Action{Hint: "r", Label: "regenerate", Priority: shared.Primary},
		shared.Action{Hint: "c", Label: "copy", Priority: shared.Primary},
		shared.Action{Hint: "V", Label: "vault", Priority: shared.Secondary},
		shared.Action{Hint: "t", Label: "themes", Priority: shared.Optional},
		shared.Action{Hint: "v", Label: "view", Priority: shared.Essential},
		shared.Action{Hint: "q", Label: "quit", Priority: shared.Essential},
	)
	for i := range actions {
		if actions[i].Key == nil && actions[i].OnClick == nil && len(actions[i].Hint) == 1 {
			actions[i].Key = tcell.NewEventKey(tcell.KeyRune, rune(actions[i].Hint[0]), 0)
		}
	}
	return shared.Footer{Actions: actions}
}

func (s *View) FooterRows(width, maxRows int) []string {
	return s.FooterActions().Rows(width-2, shared.FooterOptions{MaxRows: maxRows})
}

func (s *View) Footer(width int) string {
	return strings.Join(s.FooterRows(width, 1), "")
}
