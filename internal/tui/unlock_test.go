package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestUnlockPromptReturnsInputWithoutTerminalSuspension(t *testing.T) {
	updates := make(chan func(), 1)
	s := &screen{dispatch: func(_ context.Context, fn func()) { updates <- fn }}
	result := make(chan string, 1)
	go func() {
		value, err := s.requestPassphrase(context.Background())
		if err != nil {
			t.Error(err)
		}
		result <- value
	}()
	select {
	case update := <-updates:
		update()
	case <-time.After(time.Second):
		t.Fatal("prompt not dispatched")
	}
	for _, r := range "fixture" {
		s.handleUnlock(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	s.handleUnlock(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	select {
	case value := <-result:
		if value != "fixture" {
			t.Fatal("input changed")
		}
	case <-time.After(time.Second):
		t.Fatal("prompt blocked")
	}
	if s.unlock != nil {
		t.Fatal("prompt retained after submit")
	}
}

func TestUnlockInputUsesConsistentByteLimit(t *testing.T) {
	p := &unlockPrompt{}
	p.append("one\ntwo\x00\t")
	if p.value != "onetwo" {
		t.Fatal("control characters accepted")
	}
	p.value = strings.Repeat("x", maxPassphraseBytes-1)
	p.append("世")
	if len(p.value) != maxPassphraseBytes-1 {
		t.Fatal("multibyte rune exceeded limit")
	}
	p.append("a")
	if len(p.value) != maxPassphraseBytes {
		t.Fatal("limit rejected valid final byte")
	}
}

func TestCanceledUnlockRemovesPopup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	updates := make(chan func(), 2)
	s := &screen{operationContext: context.Background(), dispatch: func(_ context.Context, fn func()) { updates <- fn }}
	finished := make(chan error, 1)
	go func() { _, err := s.requestPassphrase(ctx); finished <- err }()
	(<-updates)()
	s.unlock.value = "fixture"
	cancel()
	select {
	case err := <-finished:
		if err != context.Canceled {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("prompt did not cancel")
	}
	select {
	case cleanup := <-updates:
		cleanup()
	case <-time.After(time.Second):
		t.Fatal("cleanup not dispatched")
	}
	if s.unlock != nil {
		t.Fatal("canceled popup retained")
	}
}
