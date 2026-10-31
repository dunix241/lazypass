package tui

import (
	"time"
	"unicode/utf8"

	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

type notificationLevel = shared.Level

const (
	notificationInfo    = shared.Info
	notificationSuccess = shared.Success
	notificationWarning = shared.Warning
	notificationError   = shared.Error
)

func (s *screen) notify(level notificationLevel, message string) {
	s.status, s.statusLevel, s.statusTill = message, level, time.Now().Add(2*time.Second)
}

func (s *screen) drawHeaderStatus(screen tcell.Screen, logo logoLayout, x, y, width int) {
	if s.status != "" && time.Now().Before(s.statusTill) {
		statusX := x + width - utf8.RuneCountInString(s.status) - 2
		if statusX > x+logo.right()+2 {
			shared.PrintAt(screen, statusX, y, s.status, s.notificationColor(s.statusLevel))
		} else {
			rightWidth, leftWidth := width-logo.right()-2, logo.x-2
			if rightWidth >= leftWidth {
				shared.PrintAt(screen, x+logo.right()+1, y, shared.Truncate(s.status, rightWidth), s.notificationColor(s.statusLevel))
			} else {
				shared.PrintAt(screen, x+1, y, shared.Truncate(s.status, leftWidth), s.notificationColor(s.statusLevel))
			}
		}
	}
}

func (s *screen) notificationColor(level notificationLevel) tcell.Color {
	switch level {
	case notificationSuccess:
		return s.colors.Accent
	case notificationWarning:
		return s.colors.Warning
	case notificationError:
		return tcell.ColorRed
	default:
		return s.colors.Focus
	}
}
