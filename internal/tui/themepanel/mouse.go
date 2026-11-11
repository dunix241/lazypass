package themepanel

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"lazypass/internal/tui/shared"
)

func (s *Panel) HandleMouse(action tview.MouseAction, event *tcell.EventMouse, frame shared.Frame) {
	r := themePanelRect(frame.Viewport)
	if r.W < 28 || r.H < 8 {
		return
	}
	x, y := event.Position()
	if !r.Contains(x, y) {
		return
	}
	if action == tview.MouseLeftClick && s.error == "" {
		if item, ok := (shared.Footer{Actions: s.footerActions()}).HitAction(shared.Rect{X: r.X + 2, Y: r.Y + r.H - 2, W: r.W - 4, H: 1}, shared.FooterOptions{MaxRows: 1}, x, y); ok {
			s.HandleKey(item.Key)
			return
		}
	}
	if s.mode != themePicker && s.mode != themeEditor {
		return
	}
	if action == tview.MouseScrollUp || action == tview.MouseScrollDown {
		key := tcell.KeyDown
		if action == tview.MouseScrollUp {
			key = tcell.KeyUp
		}
		s.HandleKey(tcell.NewEventKey(key, 0, 0))
		return
	}
	if action != tview.MouseLeftClick && action != tview.MouseLeftDoubleClick {
		return
	}
	if x < r.X+2 || x >= r.X+r.W-2 {
		return
	}
	if s.mode == themeEditor {
		start, end := s.roleViewport.Window(len(themeRoles), r.H-5, s.role)
		index := start + y - r.Y - 3
		if y >= r.Y+3 && index >= start && index < end {
			s.role = index
			s.colorText = *paletteRole(&s.pending, index)
			s.colorEdits = false
		}
		return
	}
	matches, listY, start, end := s.pickerWindow(r)
	index := start + y - listY
	if y < listY || index < start || index >= end {
		return
	}
	s.selected = matches[index]
	if s.previewTheme(s.names[s.selected]) {
		s.previewed = true
	}
	if action == tview.MouseLeftDoubleClick {
		s.applyThemeSelection()
	}
}
