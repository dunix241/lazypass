package vault

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

type folderVault struct {
	memoryVault
	folder vault.Path
}

func (p *folderVault) CreateFolder(_ context.Context, path vault.Path) error {
	p.folder = append(vault.Path(nil), path...)
	return p.err
}

func TestManualAddFolderAndNestedValue(t *testing.T) {
	p := &folderVault{}
	prompted := false
	saved := 0
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}, Password: func() string { t.Fatal("manual Add used Generator"); return "" }, RunPrompted: func(fn func(context.Context) error) error { prompted = true; return fn(context.Background()) }, Saved: func(vault.Path) { saved++ }})
	s.OpenAdd(vault.Path{"personal"})
	storeKey(s, tcell.KeyEnter, 0)
	for _, r := range "mail/" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyEnter, 0)
	if s.IsOpen() || p.folder.String() != "personal/mail" || saved != 1 {
		t.Fatal("folder creation failed")
	}
	s.OpenAdd(vault.Path{"personal"})
	storeKey(s, tcell.KeyEnter, 0)
	for _, r := range "mail/account" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyEnter, 0)
	if !s.editingValue || p.store.Password != "" {
		t.Fatal("entry path should advance to value")
	}
	for _, r := range "existing-secret" {
		storeKey(s, tcell.KeyRune, r)
	}
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(80, 24)
	f := testFrame()
	f.Viewport.W, f.Viewport.H = 80, 24
	s.Draw(sim, f)
	var output strings.Builder
	for y := 0; y < 24; y++ {
		for x := 0; x < 80; x++ {
			cell, _, _ := sim.Get(x, y)
			output.WriteString(cell)
		}
	}
	if strings.Contains(output.String(), "existing-secret") || !strings.Contains(output.String(), "********") {
		t.Fatal("value not masked")
	}
	storeKey(s, tcell.KeyEnter, 0)
	if s.IsOpen() || s.value != "" || p.store.Password != "existing-secret" || p.store.Path.String() != "personal/mail/account" || !prompted || saved != 2 {
		t.Fatal("manual value save failed")
	}
}

func TestManualAddCancelAndEditing(t *testing.T) {
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: &memoryVault{}}})
	s.OpenAdd(nil)
	storeKey(s, tcell.KeyEnter, 0)
	for _, r := range "entry" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyEnter, 0)
	if !s.IsOpen() || !s.editingValue {
		t.Fatal("value editor did not open")
	}
	for _, r := range "secret" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyBacktab, 0)
	if s.value != "" || s.editingValue {
		t.Fatal("return to path retained secret")
	}
	storeKey(s, tcell.KeyEnter, 0)
	for _, r := range "new secret" {
		storeKey(s, tcell.KeyRune, r)
	}
	storeKey(s, tcell.KeyEscape, 0)
	if s.IsOpen() || s.value != "" {
		t.Fatal("cancel retained secret")
	}
}
