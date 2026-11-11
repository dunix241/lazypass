package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
	"lazypass/internal/tui/input"
	"lazypass/internal/tui/shared"
)

type keybindingsState struct {
	actions   []shared.Action
	filter    input.Editor
	filtering bool
	selected  int
	viewport  shared.ListViewport
}

var keybindingsFilterOptions = input.Options{MaxBytes: 256}

func (s *screen) openKeybindings() {
	if s.overlay != noOverlay {
		return
	}
	actions := append([]shared.Action(nil), s.viewActions().Actions...)
	actions = append(actions, shared.Action{Hint: "z", Label: "compact mode"}, shared.Action{Hint: "?", Label: "keybindings"})
	if s.route == VaultRoute {
		actions = append(actions, shared.Action{Hint: "c", Label: "copy entry value / folder name"})
		if !s.vault.HasStagedTransfers() {
			actions = append(actions, shared.Action{Hint: "p", Label: "paste staged items"})
		}
		actions = append(actions, shared.Action{Hint: "j/k · ↑/↓", Label: "navigate"}, shared.Action{Hint: "Enter", Label: "open folder / copy entry"}, shared.Action{Hint: "Esc", Label: "clear marks and staged items"})
	}
	s.keybindings = keybindingsState{actions: actions}
	s.openOverlay(keybindingsOverlay, func() bool { return true })
}

func (k *keybindingsState) filteredActions() []shared.Action {
	query := strings.ToLower(k.filter.Value())
	if query == "" {
		return k.actions
	}
	var actions []shared.Action
	for _, action := range k.actions {
		if strings.Contains(strings.ToLower(action.Hint+" "+action.Label), query) {
			actions = append(actions, action)
		}
	}
	return actions
}

func (k *keybindingsState) moveSelection(delta int) {
	k.selected = max(0, min(len(k.filteredActions())-1, k.selected+delta))
}

func (k *keybindingsState) paste(text string) {
	if k.filtering {
		_ = k.filter.Insert(text, keybindingsFilterOptions)
		k.selected = 0
	}
}

func (s *screen) handleKeybindings(event *tcell.EventKey) {
	k := &s.keybindings
	if event.Key() == tcell.KeyEscape {
		if k.filtering || k.filter.Value() != "" {
			k.filter = input.Editor{}
			k.filtering = false
			k.selected = 0
			return
		}
		s.overlay = noOverlay
		return
	}
	if k.filtering {
		if event.Key() == tcell.KeyEnter {
			k.filtering = false
			return
		}
		_, _ = k.filter.HandleKey(event, keybindingsFilterOptions)
		k.selected = 0
		return
	}
	switch event.Key() {
	case tcell.KeyUp:
		k.moveSelection(-1)
	case tcell.KeyDown:
		k.moveSelection(1)
	case tcell.KeyRune:
		switch event.Rune() {
		case '/':
			k.filtering = true
		case 'z':
			s.toggleCompact()
		case 'j':
			k.moveSelection(1)
		case 'k':
			k.moveSelection(-1)
		case '?', 'q':
			s.overlay = noOverlay
		}
	}
}

func (s *screen) drawKeybindings(screen tcell.Screen, frame shared.Frame) {
	k := &s.keybindings
	r := shared.CenteredPanel(frame.Viewport, 70, min(24, frame.Viewport.H-2))
	mode := "off"
	if s.cfg.Compact {
		mode = "on"
	}
	shared.Panel{Bounds: r, Title: "Keybindings · compact " + mode, Border: frame.Palette.Focus, TitleColor: frame.Palette.Focus, Background: frame.Palette.Surface, Footer: "/ filter  •  z compact  •  <esc> back", FooterColor: frame.Palette.Muted}.Draw(screen, func(content shared.Rect) {
		style := tcell.StyleDefault.Foreground(frame.Palette.Text).Background(frame.Palette.Surface)
		if k.filtering {
			shared.PrintAt(screen, content.X, content.Y, "/ ", frame.Palette.Muted)
			k.filter.Draw(screen, content.X+2, content.Y, max(0, content.W-2), 1, style, true, keybindingsFilterOptions)
		} else {
			shared.PrintAt(screen, content.X, content.Y, shared.Truncate("Filter: "+k.filter.Value(), content.W), frame.Palette.Muted)
		}
		actions := k.filteredActions()
		keyWidth := 0
		for _, action := range k.actions {
			keyWidth = max(keyWidth, uniseg.StringWidth(action.Hint))
		}
		keyWidth = min(keyWidth, max(0, content.W/2-1))
		labelX := content.X + keyWidth + 1
		start, end := k.viewport.Window(len(actions), max(0, content.H-2), k.selected)
		if len(actions) == 0 {
			shared.PrintAt(screen, content.X, content.Y+2, "No matching keybindings", frame.Palette.Muted)
		}
		for i := start; i < end; i++ {
			rowStyle := style
			if i == k.selected {
				rowStyle = rowStyle.Foreground(frame.Palette.Focus)
			}
			y := content.Y + 2 + i - start
			hint := shared.Truncate(actions[i].Hint, keyWidth)
			shared.PrintStyled(screen, content.X+keyWidth-uniseg.StringWidth(hint), y, hint, rowStyle)
			shared.PrintStyled(screen, labelX, y, shared.Truncate(actions[i].Label, max(0, content.W-keyWidth-1)), rowStyle)
		}
	})
}

func (s *screen) handleKeybindingsMouse(action tview.MouseAction, event *tcell.EventMouse) {
	k := &s.keybindings
	v := s.frame().Viewport
	r := shared.CenteredPanel(v, 70, min(24, v.H-2))
	x, y := event.Position()
	if !r.Contains(x, y) {
		return
	}
	switch action {
	case tview.MouseScrollUp:
		k.moveSelection(-1)
	case tview.MouseScrollDown:
		k.moveSelection(1)
	case tview.MouseLeftClick:
		start, end := k.viewport.Window(len(k.filteredActions()), max(0, r.H-6), k.selected)
		index := start + y - r.Y - 3
		if y >= r.Y+3 && index >= start && index < end {
			k.selected = index
		}
	}
}
