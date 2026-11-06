package tui

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
	"lazypass/internal/clipboard"
	"lazypass/internal/debug"
	"lazypass/internal/task"
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
	s.notifyWithDuration(level, text, 0)
}

func (s *screen) notifyWithDuration(level notificationLevel, text string, duration time.Duration) {
	minimum := 3 * time.Second
	if level == shared.Warning || level == shared.Error {
		minimum = 8 * time.Second
	}
	s.notifications.Show(notification.Message{ID: "operation", Text: text, Level: level, Duration: duration,
		DurationMode: notification.DynamicDuration, ReadingTime: &notification.ReadingTime{Minimum: minimum}}, time.Now())
}

func (s *screen) notifyRecovery(text, command string) {
	s.notifications.Show(notification.Message{ID: "vault recovery", Text: text, Level: shared.Warning, Duration: -1,
		Actions: []notification.Action{{Label: "copy login command", KeepOpen: true, Run: func() {
			if s.tasks == nil {
				return
			}
			_, err := s.tasks.Submit(task.Task{ID: "copy recovery command", Run: func(ctx context.Context) error {
				err := clipboard.Copy(command)
				s.dispatch(ctx, func() {
					if err != nil {
						s.notify(shared.Error, "Could not copy login command")
					} else {
						s.notifications.Remove("vault recovery", time.Now())
						s.notify(shared.Success, "Login command copied; run it in your terminal, then refresh")
					}
				})
				return nil
			}})
			if err != nil && err != task.ErrDuplicate {
				debug.Failure("recovery copy schedule", debug.Unavailable)
			}
		}}, {Label: "dismiss"}},
	}, time.Now())
}

func (s *screen) drawHeaderStatus(screen tcell.Screen, logo logoLayout, x, y, width int) {
	s.notificationBounds = nil
	s.drawnNotification = nil
	message := s.notifications.Current()
	if message == nil {
		return
	}
	available, offset, alignRight := notificationSpace(logo, width)
	start := x + offset
	if available <= 0 {
		return
	}
	fallback := tcell.StyleDefault.Foreground(s.notificationColor(message.Level)).Background(s.colors.Base).Bold(true)
	presentation := notification.Format(*message, fallback)
	_, _, _, height := s.GetRect()
	lines := notificationLines(presentation.Text, available, height)
	if len(lines) == 0 {
		return
	}
	if alignRight {
		blockWidth := 0
		for _, line := range lines {
			blockWidth = max(blockWidth, uniseg.StringWidth(line))
		}
		start = x + width - blockWidth - 2
	}
	for row, line := range lines {
		textWidth := uniseg.StringWidth(line)
		shared.PrintStyled(screen, start, y+row, line, presentation.Style)
		if len(message.Actions) > 0 {
			s.notificationBounds = append(s.notificationBounds, shared.Rect{X: start, Y: y + row, W: textWidth, H: 1})
		}
	}
	if s.loggedNotification != message {
		s.loggedNotification = message
		if message.ID == "update" {
			debug.Event("update notification rendered")
		}
	}
	s.drawnNotification = message
}

func notificationSpace(logo logoLayout, width int) (available, offset int, alignRight bool) {
	if right := width - logo.right() - 3; right > 0 {
		return min(right, 64), logo.right() + 1, true
	}
	return logo.x - 2, 1, false
}

func notificationLines(text string, width, height int) []string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' {
			return ' '
		}
		return r
	}, text)
	paragraphs := strings.Split(text, "\n")
	var lines []string
	for _, paragraph := range paragraphs {
		lines = append(lines, shared.WrapText(paragraph, width)...)
	}
	limit := max(1, min(6, height-12))
	if len(lines) > limit {
		lines = lines[:limit]
		lines[limit-1] = shared.Truncate(lines[limit-1]+" …", width)
		if len(paragraphs) > 1 && strings.HasPrefix(paragraphs[len(paragraphs)-1], "F2 ") {
			lines[limit-1] = shared.Truncate(paragraphs[len(paragraphs)-1], width)
		}
	}
	return lines
}

func (s *screen) notificationRows(width, height int) int {
	message := s.notifications.Current()
	if message == nil {
		return 0
	}
	logo := calculateLogoLayout(width, height)
	available, _, _ := notificationSpace(logo, width)
	presentation := notification.Format(*message, tcell.StyleDefault)
	return len(notificationLines(presentation.Text, available, height))
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
