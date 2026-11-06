package notification

import (
	"math"
	"time"
	"unicode"
)

type DurationMode uint8

const (
	FixedDuration DurationMode = iota
	DynamicDuration
)

const (
	DefaultWordsPerSecond  = 3.0
	DefaultMaximumDuration = 20 * time.Second
)

// ReadingTime controls dynamic durations. Zero fields use the defaults.
type ReadingTime struct {
	WordsPerSecond float64
	Minimum        time.Duration
	Maximum        time.Duration
}

func (r ReadingTime) Duration(text string) time.Duration {
	return r.durationForWords(wordCount(text))
}

func wordCount(text string) int {
	words, inWord := 0, false
	for _, character := range text {
		if unicode.IsSpace(character) {
			inWord = false
		} else if !inWord {
			words++
			inWord = true
		}
	}
	return words
}

func (r ReadingTime) durationForWords(words int) time.Duration {
	speed := r.WordsPerSecond
	if speed <= 0 || math.IsNaN(speed) || math.IsInf(speed, 0) {
		speed = DefaultWordsPerSecond
	}
	minimum, maximum := r.Minimum, r.Maximum
	if minimum <= 0 {
		minimum = DefaultDuration
	}
	if maximum <= 0 {
		maximum = DefaultMaximumDuration
	}
	maximum = max(minimum, maximum)
	seconds := float64(words) / speed
	if seconds >= maximum.Seconds() {
		return maximum
	}
	return max(minimum, time.Duration(math.Ceil(seconds*float64(time.Second))))
}

func (m *Manager) messageDuration(message Message) time.Duration {
	if message.Duration != 0 {
		return message.Duration
	}
	if message.DurationMode == DynamicDuration {
		reading := m.options.ReadingTime
		if message.ReadingTime != nil {
			reading = *message.ReadingTime
		}
		words := wordCount(message.Title) + wordCount(message.Text)
		for _, action := range message.Actions {
			words += wordCount(action.Label)
		}
		return reading.durationForWords(words)
	}
	if m.options.DefaultDuration != 0 {
		return m.options.DefaultDuration
	}
	return DefaultDuration
}
