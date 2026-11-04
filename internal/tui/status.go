package tui

import (
	"strings"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
	"lazypass/internal/debug"
	"lazypass/internal/tui/notification"
	"lazypass/internal/tui/shared"
)

type notificationLevel = shared.Level

const (
	notificationInfo    = shared.Info
	notificationSuccess = shared.Success
	notificationWarning = shared.Warning
	notificationError   = shared.Error
)

func (s *screen) notify(level notificationLevel, text string) {
	s.notifications.Show(notification.Message{ID: "operation", Text: text, Level: level}, time.Now())
}

func (s *screen) drawHeaderStatus(screen tcell.Screen, logo logoLayout, x, y, width int) {
	s.notificationBounds = nil
	s.drawnNotification = nil
	message := s.notifications.Current()
	if message == nil {
		return
	}
	right, left := width-logo.right()-3, logo.x-2
	available, start := right, x+logo.right()+1
	if right <= 0 {
		available, start = left, x+1
	}
	if available <= 0 {
		return
	}
	fallback := tcell.StyleDefault.Foreground(s.notificationColor(message.Level)).Background(s.colors.Base).Bold(true)
	presentation := notification.Format(*message, fallback)
	text := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, presentation.Text)
	text = shared.Truncate(text, available)
	textWidth := uniseg.StringWidth(text)
	if textWidth == 0 {
		return
	}
	if available == right {
		start = x + width - textWidth - 2
	}
	foreground, _, _ := presentation.Style.Decompose()
	shared.PrintAt(screen, start, y, text, foreground)
	for column := start; column < start+textWidth; column++ {
		main, combining, _, _ := screen.GetContent(column, y)
		screen.SetContent(column, y, main, combining, presentation.Style)
	}
	if s.loggedNotification != message {
		s.loggedNotification = message
		if message.ID == "update" {
			debug.Event("update notification rendered")
		}
	}
	s.drawnNotification = message
	// A click activates the first action; additional actions use F3, F4, etc.
	if len(message.Actions) > 0 {
		s.notificationBounds = []shared.Rect{{X: start, Y: y, W: textWidth, H: 1}}
	}
}

func (s *screen) activateNotification(index int) bool {
	if s.drawnNotification == nil || s.drawnNotification != s.notifications.Current() {
		return false
	}
	return s.notifications.Activate(index, time.Now())
}

func (s *screen) notificationColor(level notificationLevel) tcell.Color {
	switch level {
	case shared.Success:
		return s.colors.Accent
	case shared.Warning:
		return s.colors.Warning
	case shared.Error:
		return tcell.ColorRed
	default:
		return s.colors.Focus
	}
}
