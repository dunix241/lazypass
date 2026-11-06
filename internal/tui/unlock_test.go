package tui

import (
	"context"
	"lazypass/internal/vault/service"
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
		value, err := s.requestVaultPassphrase(context.Background(), nil)
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

func TestInactiveVaultCannotOpenUnlockPrompt(t *testing.T) {
	updates := make(chan func(), 1)
	first, second := &memoryVault{}, &memoryVault{}
	s := &screen{vaults: []NamedVault{{Service: service.Service{Provider: first}}, {Service: service.Service{Provider: second}}}, vaultIndex: 1, dispatch: func(_ context.Context, fn func()) { updates <- fn }}
	finished := make(chan error, 1)
	go func() { _, err := s.requestVaultPassphrase(context.Background(), first); finished <- err }()
	select {
	case update := <-updates:
		update()
	case <-time.After(time.Second):
		t.Fatal("prompt not dispatched")
	}
	select {
	case err := <-finished:
		if err != context.Canceled {
			t.Fatalf("inactive prompt: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("inactive prompt blocked")
	}
	if s.unlock != nil {
		t.Fatal("inactive vault displayed prompt")
	}
}

func TestUnlockInputUsesConsistentByteLimit(t *testing.T) {
	p := &unlockPrompt{}
	p.append("one\ntwo\x00\t")
	if string(p.value) != "onetwo" {
		t.Fatal("control characters accepted")
	}
	p.value = []byte(strings.Repeat("x", maxPassphraseBytes-1))
	p.append("€")
	if len(p.value) != maxPassphraseBytes-1 {
		t.Fatal("multibyte rune exceeded limit")
	}
	p.append("a")
	if len(p.value) != maxPassphraseBytes {
		t.Fatal("limit rejected valid final byte")
	}
}

func TestUnlockBufferClearedOnCancelAndDeletion(t *testing.T) {
	p := &unlockPrompt{reply: make(chan unlockReply, 1)}
	p.append("fixture")
	buffer := p.value
	p.truncate(3)
	for _, b := range buffer[3:] {
		if b != 0 {
			t.Fatal("deleted bytes retained")
		}
	}
	s := &screen{unlock: p}
	s.finishUnlock(unlockReply{err: context.Canceled})
	for _, b := range buffer {
		if b != 0 {
			t.Fatal("cancelled bytes retained")
		}
	}
}

func TestUnlockDiscardClearsQueuedSecret(t *testing.T) {
	buffer := []byte("public-fixture")
	p := &unlockPrompt{reply: make(chan unlockReply, 1)}
	p.reply <- unlockReply{value: buffer}
	p.discardReply()
	for _, b := range buffer {
		if b != 0 {
			t.Fatal("queued secret retained")
		}
	}
	select {
	case <-p.reply:
		t.Fatal("reply not drained")
	default:
	}
}

func TestUnlockWordDeletionClearsRemovedBytes(t *testing.T) {
	p := &unlockPrompt{}
	p.append("folder/word  ")
	buffer := p.value
	p.deleteWord()
	if string(p.value) != "folder/" {
		t.Fatal("word deletion changed")
	}
	for _, b := range buffer[len(p.value):] {
		if b != 0 {
			t.Fatal("removed word retained")
		}
	}
}

func TestCanceledUnlockRemovesPopup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	updates := make(chan func(), 2)
	s := &screen{operationContext: context.Background(), dispatch: func(_ context.Context, fn func()) { updates <- fn }}
	finished := make(chan error, 1)
	go func() { _, err := s.requestVaultPassphrase(ctx, nil); finished <- err }()
	(<-updates)()
	s.unlock.value = []byte("fixture")
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
