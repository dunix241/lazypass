// Package notification manages queued, timed notifications on the UI thread.
package notification

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/tui/shared"
)

const DefaultDuration = 3 * time.Second
const maxPending = 32

type Action struct {
	ID       string
	Label    string
	Run      func()
	KeepOpen bool
}

type CloseReason uint8

const (
	Dismissed CloseReason = iota
	Expired
	Activated
	Removed
)

type Presentation struct {
	Text  string
	Style tcell.Style
}

type Presenter func(Message, tcell.Style) Presentation

type Message struct {
	ID       string
	Text     string
	Level    shared.Level
	Duration time.Duration
	// A nonzero Duration overrides DurationMode; negative durations stay open.
	DurationMode DurationMode
	ReadingTime  *ReadingTime
	Actions      []Action
	Title        string
	Icon         string
	Style        *tcell.Style
	Present      Presenter
	OnClose      func(CloseReason)
}

type Options struct {
	ReadingTime     ReadingTime
	DefaultDuration time.Duration
	MaxPending      int
}

type Manager struct {
	extra     []*Manager
	current   *Message
	queue     []Message
	until     time.Time
	options   Options
	paused    bool
	remaining time.Duration
}

func New(options Options) *Manager { return &Manager{options: options} }

func (m *Manager) Show(message Message, now time.Time) {
	if message.Text == "" && message.Title == "" && message.Present == nil {
		return
	}
	message = m.prepare(message)
	if message.ID != "" {
		for i := range m.queue {
			if m.queue[i].ID == message.ID {
				m.queue[i] = message
				return
			}
		}
	}
	if len(m.extra) > 0 {
		if message.ID != "" {
			for _, lane := range m.extra {
				if lane.current != nil && lane.current.ID == message.ID {
					lane.start(message, now)
					return
				}
			}
		}
		if m.current != nil && (message.ID == "" || m.current.ID != message.ID) {
			for _, lane := range m.extra {
				if lane.current == nil {
					lane.start(message, now)
					return
				}
			}
		}
	}
	if m.current != nil && message.ID != "" && m.current.ID == message.ID {
		m.start(message, now)
		return
	}
	if m.current == nil {
		m.start(message, now)
		return
	}
	limit := m.options.MaxPending
	if limit <= 0 {
		limit = maxPending
	}
	if len(m.queue) >= limit {
		copy(m.queue, m.queue[1:])
		m.queue[len(m.queue)-1] = Message{}
		m.queue = m.queue[:len(m.queue)-1]
	}
	m.queue = append(m.queue, message)
}

func (m *Manager) prepare(message Message) Message {
	message.Duration = m.messageDuration(message)
	message.ReadingTime = nil
	message.Actions = append([]Action(nil), message.Actions...)
	if message.Style != nil {
		style := *message.Style
		message.Style = &style
	}
	return message
}

func (m *Manager) start(message Message, now time.Time) {
	m.current = &message
	m.paused = false
	m.remaining = 0
	m.until = time.Time{}
	if message.Duration > 0 {
		m.until = now.Add(message.Duration)
	}
}

func (m *Manager) Current() *Message { return m.current }
func (m *Manager) Pending() int      { return len(m.queue) }
func (m *Manager) Deadline() time.Time {
	deadline := m.until
	for _, lane := range m.extra {
		if d := lane.Deadline(); !d.IsZero() && (deadline.IsZero() || d.Before(deadline)) {
			deadline = d
		}
	}
	return deadline
}
func (m *Manager) Tick(now time.Time) bool {
	changed := false
	for _, lane := range m.extra {
		changed = lane.Tick(now) || changed
	}
	if m.current == nil || m.until.IsZero() || now.Before(m.until) {
		m.fillSlots(now)
		return changed
	}
	m.close(now, Expired)
	m.fillSlots(now)
	return true
}

func (m *Manager) fillSlots(now time.Time) {
	if m.current == nil && len(m.queue) > 0 {
		next := m.queue[0]
		m.queue[0] = Message{}
		m.queue = m.queue[1:]
		m.start(next, now)
	}
	for _, lane := range m.extra {
		if lane.current == nil && len(m.queue) > 0 {
			next := m.queue[0]
			m.queue[0] = Message{}
			m.queue = m.queue[1:]
			lane.start(next, now)
		}
	}
}

func (m *Manager) Dismiss(now time.Time) {
	m.close(now, Dismissed)
	m.fillSlots(now)
}

func (m *Manager) close(now time.Time, reason CloseReason) {
	previous := m.current
	m.current = nil
	m.until = time.Time{}
	m.paused = false
	m.remaining = 0
	if len(m.queue) > 0 {
		next := m.queue[0]
		m.queue[0] = Message{}
		m.queue = m.queue[1:]
		m.start(next, now)
	}
	if previous != nil && previous.OnClose != nil {
		previous.OnClose(reason)
	}
}

func (m *Manager) Activate(index int, now time.Time) bool {
	if m.current == nil || index < 0 || index >= len(m.current.Actions) {
		return false
	}
	action := m.current.Actions[index]
	if !action.KeepOpen {
		m.close(now, Activated)
	}
	if action.Run != nil {
		action.Run()
	}
	return true
}

func (m *Manager) Pause(now time.Time) {
	for _, lane := range m.extra {
		lane.Pause(now)
	}
	if m.current == nil || m.paused || m.until.IsZero() {
		return
	}
	m.remaining = max(0, m.until.Sub(now))
	m.until = time.Time{}
	m.paused = true
}

func (m *Manager) Resume(now time.Time) {
	for _, lane := range m.extra {
		lane.Resume(now)
	}
	if !m.paused {
		return
	}
	m.paused = false
	m.until = now.Add(m.remaining)
}

func (m *Manager) Remove(id string, now time.Time) bool {
	if id == "" {
		return false
	}
	for _, lane := range m.extra {
		if lane.Remove(id, now) {
			m.fillSlots(now)
			return true
		}
	}
	for i, message := range m.queue {
		if message.ID != id {
			continue
		}
		copy(m.queue[i:], m.queue[i+1:])
		m.queue[len(m.queue)-1] = Message{}
		m.queue = m.queue[:len(m.queue)-1]
		if message.OnClose != nil {
			message.OnClose(Removed)
		}
		return true
	}
	if m.current != nil && m.current.ID == id {
		m.close(now, Removed)
		m.fillSlots(now)
		return true
	}
	return false
}

func (m *Manager) SetSlots(slots int) {
	slots = max(1, min(slots, 8))
	if len(m.extra) > slots-1 {
		for _, lane := range m.extra[slots-1:] {
			if lane.current != nil {
				m.queue = append(m.queue, *lane.current)
			}
			m.queue = append(m.queue, lane.queue...)
		}
		m.extra = m.extra[:slots-1]
	}
	for len(m.extra) < slots-1 {
		m.extra = append(m.extra, New(m.options))
	}
	m.fillSlots(time.Now())
}

func (m *Manager) Visible() []*Message {
	var messages []*Message
	if m.current != nil {
		messages = append(messages, m.current)
	}
	for _, lane := range m.extra {
		if lane.current != nil {
			messages = append(messages, lane.current)
		}
	}
	return messages
}

func (m *Manager) ActivateMessage(message *Message, index int, now time.Time) bool {
	if m.current == message {
		activated := m.Activate(index, now)
		m.fillSlots(now)
		return activated
	}
	for _, lane := range m.extra {
		if lane.current == message {
			activated := lane.Activate(index, now)
			m.fillSlots(now)
			return activated
		}
	}
	return false
}
