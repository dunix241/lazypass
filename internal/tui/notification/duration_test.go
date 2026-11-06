package notification

import (
	"strings"
	"testing"
	"time"
)

func TestDynamicDurationAndOverrides(t *testing.T) {
	text := strings.Repeat("word ", 45)
	for _, tc := range []struct {
		name    string
		message Message
		want    time.Duration
	}{
		{"dynamic", Message{Text: text, DurationMode: DynamicDuration}, 15 * time.Second},
		{"fixed override", Message{Text: text, DurationMode: DynamicDuration, Duration: time.Second}, time.Second},
		{"sticky override", Message{Text: text, DurationMode: DynamicDuration, Duration: -1}, -1},
		{"default fixed", Message{Text: text}, DefaultDuration},
		{"custom speed", Message{Text: text, DurationMode: DynamicDuration, ReadingTime: &ReadingTime{WordsPerSecond: 9}}, 5 * time.Second},
		{"minimum", Message{Text: "short", DurationMode: DynamicDuration}, DefaultDuration},
		{"maximum", Message{Text: strings.Repeat("word ", 1000), DurationMode: DynamicDuration}, 20 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(Options{})
			now := time.Now()
			m.Show(tc.message, now)
			if got := m.Current().Duration; got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			if tc.want < 0 && !m.Deadline().IsZero() {
				t.Fatal("sticky notification has deadline")
			}
		})
	}
}

func TestDynamicDurationIncludesActionLabels(t *testing.T) {
	m := New(Options{ReadingTime: ReadingTime{WordsPerSecond: 1, Minimum: time.Second}})
	m.Show(Message{Title: "Title", Text: "Body", DurationMode: DynamicDuration, Actions: []Action{{Label: "copy command"}}}, time.Now())
	if m.Current().Duration != 4*time.Second {
		t.Fatal("reading time omitted title or actions")
	}
}
