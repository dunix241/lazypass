package config

import "testing"

func TestMessageDefaultsAndExplicitSlots(t *testing.T) {
	m := (Messages{}).Resolved()
	if m.Notifications.Slots != 2 || m.Status.Slots != 1 || Defaults().Messages != m {
		t.Fatal("message defaults differ")
	}
	m.Notifications.Slots = 1
	m.Status.Slots = 3
	if got := m.Resolved(); got.Notifications.Slots != 1 || got.Status.Slots != 3 {
		t.Fatal("explicit slot counts overwritten")
	}
}
