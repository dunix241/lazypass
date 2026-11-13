package tui

import (
	"lazypass/internal/config"
	"lazypass/internal/tui/shared"
	"testing"
)

func TestNotificationsAndOperationStatusAreIndependent(t *testing.T) {
	s := NewApp(config.Defaults(), "", nil).screen
	s.notifyWithDuration(shared.Info, "Renaming…", -1)
	s.notify(shared.Error, "Authentication failed")
	if s.notifications.Current().Text != "Authentication failed" || s.statuses.Current().Text != "Renaming…" {
		t.Fatal("notification replaced operation status")
	}
	s.notifyWithDuration(shared.Info, "Refreshing…", -1)
	if s.statuses.Current().Text != "Refreshing…" || s.notifications.Pending() != 0 {
		t.Fatal("progress entered notification queue")
	}
}
