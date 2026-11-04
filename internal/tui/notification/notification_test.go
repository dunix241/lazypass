package notification

import (
	"github.com/gdamore/tcell/v2"
	"strings"
	"testing"
	"time"
)

func TestPauseResumeAndCloseReasons(t *testing.T) {
	now := time.Now()
	m := New(Options{DefaultDuration: time.Second, MaxPending: 2})
	var reason CloseReason
	m.Show(Message{ID: "fixture", Text: "test", OnClose: func(value CloseReason) { reason = value }}, now)
	m.Pause(now.Add(400 * time.Millisecond))
	if m.Tick(now.Add(time.Hour)) {
		t.Fatal("paused notification expired")
	}
	m.Resume(now.Add(time.Hour))
	if m.Tick(now.Add(time.Hour + 500*time.Millisecond)) {
		t.Fatal("remaining duration lost")
	}
	if !m.Tick(now.Add(time.Hour+time.Second)) || reason != Expired {
		t.Fatal("expiry lifecycle failed")
	}
}

func TestCustomPresentationAndKeepOpenAction(t *testing.T) {
	now := time.Now()
	var m Manager
	calls := 0
	style := tcell.StyleDefault.Foreground(tcell.ColorRed).Underline(true)
	m.Show(Message{ID: "fixture", Text: "body", Title: "Title", Icon: "!", Style: &style, Actions: []Action{{ID: "retry", Label: "retry", KeepOpen: true, Run: func() { calls++ }}}}, now)
	view := Format(*m.Current(), tcell.StyleDefault)
	if !strings.Contains(view.Text, "Title: body") || view.Style != style {
		t.Fatal("style/title not preserved")
	}
	if !m.Activate(0, now) || calls != 1 || m.Current() == nil {
		t.Fatal("keep-open action closed message")
	}
	if !m.Remove("fixture", now) || m.Current() != nil {
		t.Fatal("ID removal failed")
	}
	custom := Format(Message{Present: func(_ Message, s tcell.Style) Presentation { return Presentation{Text: "custom", Style: s} }}, style)
	if custom.Text != "custom" || custom.Style != style {
		t.Fatal("custom presenter ignored")
	}
}

func TestQueuedTimeoutStartsWhenShown(t *testing.T) {
	now := time.Now()
	var m Manager
	m.Show(Message{Text: "first", Duration: time.Second}, now)
	m.Show(Message{Text: "second", Duration: 2 * time.Second}, now)
	if !m.Tick(now.Add(time.Second)) || m.Current().Text != "second" {
		t.Fatal("queue not advanced")
	}
	if m.Tick(now.Add(2 * time.Second)) {
		t.Fatal("queued duration started before presentation")
	}
	if !m.Tick(now.Add(3*time.Second)) || m.Current() != nil {
		t.Fatal("timeout did not clear notification")
	}
}

func TestIDReplacementAndActionLifecycle(t *testing.T) {
	now := time.Now()
	var m Manager
	called := 0
	m.Show(Message{ID: "operation", Text: "loading"}, now)
	m.Show(Message{ID: "operation", Text: "done"}, now)
	m.Show(Message{ID: "update", Text: "update", Actions: []Action{{Label: "dismiss", Run: func() { called++ }}}}, now)
	if m.Current().Text != "done" || m.Pending() != 1 {
		t.Fatal("ID replacement failed")
	}
	m.Dismiss(now)
	if !m.Activate(0, now) || called != 1 || m.Current() != nil {
		t.Fatal("action did not dismiss before running")
	}
	if m.Activate(0, now) {
		t.Fatal("action ran twice")
	}
}

func TestTimeoutDoesNotRunAction(t *testing.T) {
	now := time.Now()
	var m Manager
	called := false
	m.Show(Message{Text: "update", Actions: []Action{{Run: func() { called = true }}}}, now)
	m.Tick(now.Add(DefaultDuration))
	if called || m.Current() != nil {
		t.Fatal("expiry ran explicit action")
	}
}

func TestPresenterOnlyNotification(t *testing.T) {
	var manager Manager
	manager.Show(Message{Present: func(_ Message, style tcell.Style) Presentation {
		return Presentation{Text: "custom presentation", Style: style}
	}}, time.Now())
	if manager.Current() == nil {
		t.Fatal("presenter-only notification rejected")
	}
}
