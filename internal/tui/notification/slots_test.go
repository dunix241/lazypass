package notification

import (
	"testing"
	"time"
)

func TestMultipleSlotsExpireIndependentlyAndDrainQueue(t *testing.T) {
	m := New(Options{})
	m.SetSlots(2)
	now := time.Now()
	m.Show(Message{Text: "first", Duration: time.Second}, now)
	m.Show(Message{Text: "second", Duration: 2 * time.Second}, now)
	m.Show(Message{Text: "third", Duration: 3 * time.Second}, now)
	if len(m.Visible()) != 2 {
		t.Fatal("slots not filled")
	}
	m.Tick(now.Add(time.Second))
	if len(m.Visible()) != 2 || m.Current().Text != "third" {
		t.Fatal("queue not promoted")
	}
	if !m.Deadline().Equal(now.Add(2 * time.Second)) {
		t.Fatal("wrong earliest deadline")
	}
}

func TestRemovingSecondarySlotPromotesQueuedMessage(t *testing.T) {
	m := New(Options{})
	m.SetSlots(2)
	now := time.Now()
	m.Show(Message{ID: "first", Text: "first", Duration: -1}, now)
	m.Show(Message{ID: "second", Text: "second", Duration: -1}, now)
	m.Show(Message{ID: "third", Text: "third", Duration: -1}, now)
	m.Remove("second", now)
	visible := m.Visible()
	if len(visible) != 2 || visible[1].ID != "third" || m.Pending() != 0 {
		t.Fatal("free secondary slot did not promote queued message")
	}
}

func TestQueuedIDUpdateDoesNotDuplicateAfterSlotResize(t *testing.T) {
	m := New(Options{})
	now := time.Now()
	m.Show(Message{ID: "first", Text: "first", Duration: -1}, now)
	m.Show(Message{ID: "second", Text: "old", Duration: -1}, now)
	m.SetSlots(2)
	m.Show(Message{ID: "second", Text: "new", Duration: -1}, now)
	if len(m.Visible()) != 2 || m.Visible()[1].Text != "new" || m.Pending() != 0 {
		t.Fatal("duplicate message after resize")
	}
	m.SetSlots(1)
	if len(m.Visible()) != 1 || m.Pending() != 1 {
		t.Fatal("slot reduction lost message")
	}
	m.SetSlots(2)
	if len(m.Visible()) != 2 || m.Pending() != 0 {
		t.Fatal("slot increase did not restore queued message")
	}
}

func TestAllVisibleSlotsPauseTogether(t *testing.T) {
	m := New(Options{})
	m.SetSlots(2)
	now := time.Now()
	m.Show(Message{Text: "first", Duration: time.Second}, now)
	m.Show(Message{Text: "second", Duration: 2 * time.Second}, now)
	m.Pause(now)
	if !m.Deadline().IsZero() || m.Tick(now.Add(time.Minute)) {
		t.Fatal("paused slot expired")
	}
	m.Resume(now.Add(time.Minute))
	if !m.Deadline().Equal(now.Add(time.Minute + time.Second)) {
		t.Fatal("pause lost remaining lifetime")
	}
}
