package shared

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestTypedConfirmationOwnsValidationAndLifecycle(t *testing.T) {
	confirmed, cancelled := 0, 0
	c := NewConfirmation(ConfirmationOptions{RequireText: "DELETE", OnConfirm: func() { confirmed++ }, OnCancel: func() { cancelled++ }})
	enter := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	c.HandleKey(enter)
	c.Paste("DELET")
	c.HandleKey(enter)
	if confirmed != 0 {
		t.Fatal("incomplete text accepted")
	}
	c.Busy = true
	c.Paste("E")
	c.HandleKey(enter)
	if confirmed != 0 {
		t.Fatal("busy confirmation accepted input")
	}
	c.Busy = false
	c.Paste("E")
	c.HandleKey(enter)
	c.HandleKey(enter)
	if confirmed != 1 || cancelled != 0 {
		t.Fatal("confirmation callback not exactly once")
	}
}

func TestConfirmationCancelDuringPreparation(t *testing.T) {
	cancelled := false
	c := NewConfirmation(ConfirmationOptions{OnCancel: func() { cancelled = true }})
	c.Busy = true
	c.HandleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if !cancelled {
		t.Fatal("busy dialog cannot be cancelled")
	}
}

func TestFooterKeepsActionBindingsWhole(t *testing.T) {
	f := Footer{Actions: []Action{{Hint: "Enter", Label: "delete"}, {Hint: "Esc", Label: "cancel"}}}
	if got := f.TextForWidth(14); got != "Enter delete" {
		t.Fatalf("partial action rendered: %q", got)
	}
}
