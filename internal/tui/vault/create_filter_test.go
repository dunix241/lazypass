package vault

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
	"testing"
)

func TestCreateTypeFilterDoesNotAdvanceUntilSelected(t *testing.T) {
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: &memoryVault{}}})
	s.OpenAdd(nil)
	frame := testFrame()
	before, _ := s.storeFormRect(frame.Viewport)
	storeKey(s, tcell.KeyRune, 'j')
	if s.actionIndex != 1 {
		t.Fatal("j did not navigate outside filtering")
	}
	storeKey(s, tcell.KeyRune, 'k')
	storeKey(s, tcell.KeyRune, '/')
	s.Paste("import")
	actions := s.createActions()
	if !s.choosingType || len(actions) != 1 || !actions[0].bulk {
		t.Fatal("typing did not filter creation actions")
	}
	storeKey(s, tcell.KeyEscape, 0)
	if !s.IsOpen() || s.actionFilter != "" {
		t.Fatal("Escape did not clear filter")
	}
	storeKey(s, tcell.KeyRune, '/')
	s.Paste("no-match")
	after, _ := s.storeFormRect(frame.Viewport)
	if before != after {
		t.Fatal("filter changed popup geometry")
	}
	s.HandleMouse(tview.MouseScrollDown, tcell.NewEventMouse(after.X+2, after.Y+2, tcell.WheelDown, 0), frame)
	if s.actionIndex != 0 {
		t.Fatal("empty-results scrolling produced invalid selection")
	}
	storeKey(s, tcell.KeyEnter, 0)
	if !s.choosingType {
		t.Fatal("empty result selected an action")
	}
	storeKey(s, tcell.KeyCtrlU, 0)
	s.Paste("login")
	storeKey(s, tcell.KeyEnter, 0)
	if s.choosingType || s.entryType != vault.LoginEntry {
		t.Fatal("filtered entry type not selected")
	}
}
