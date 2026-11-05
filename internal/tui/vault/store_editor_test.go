package vault

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/vault"
)

func TestEntryFieldsInsertAtCursorAndKeepPosition(t *testing.T) {
	s := NewStoreForm(StoreDependencies{})
	s.open, s.editingValue, s.entryType = true, true, vault.LoginEntry
	s.value = "abcdef"
	storeKey(s, tcell.KeyLeft, 0)
	s.Paste("X")
	if s.value != "abcdeXf" {
		t.Fatalf("password insertion: %q", s.value)
	}
	storeKey(s, tcell.KeyTAB, 0)
	s.Paste("username")
	storeKey(s, tcell.KeyHome, 0)
	s.Paste("new-")
	if s.username != "new-username" {
		t.Fatalf("username insertion: %q", s.username)
	}
	storeKey(s, tcell.KeyBacktab, 0)
	s.Paste("Y")
	if s.value != "abcdeXYf" {
		t.Fatalf("field switch lost cursor: %q", s.value)
	}
}

func TestMultilineEntryInsertion(t *testing.T) {
	s := NewStoreForm(StoreDependencies{})
	s.open, s.editingValue, s.entryType = true, true, vault.NoteEntry
	s.notes = "first\nlast"
	storeKey(s, tcell.KeyHome, 0)
	s.Paste("middle\n")
	if s.notes != "first\nmiddle\nlast" {
		t.Fatalf("multiline paste: %q", s.notes)
	}
}
