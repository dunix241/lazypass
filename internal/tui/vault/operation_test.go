package vault

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

func TestEditorShowsLoadingUntilEntryArrives(t *testing.T) {
	var work func(context.Context) error
	var done func(error)
	p := &memoryVault{read: map[string]vault.Entry{"entry": {Path: vault.Path{"entry"}, Password: "fixture", Username: "user"}}}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}, RunOperation: func(w func(context.Context) error, d func(error)) { work, done = w, d }})
	s.OpenEdit(vault.Path{"entry"})
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(80, 24)
	s.Draw(sim, testFrame())
	var text strings.Builder
	for y := 0; y < 24; y++ {
		for x := 0; x < 80; x++ {
			cell, _, _ := sim.Get(x, y)
			text.WriteString(cell)
		}
	}
	if !strings.Contains(text.String(), "Edit entry") || !strings.Contains(text.String(), "Loading entry…") || strings.Contains(text.String(), "Password:") || strings.Contains(text.String(), "Add to vault") {
		t.Fatalf("incorrect loading presentation: %q", text.String())
	}
	done(work(context.Background()))
	if s.loadingEntry || s.value != "fixture" || s.username != "user" {
		t.Fatal("loaded entry not presented")
	}
}

func TestRefreshReturnsBeforeSlowOperationCompletes(t *testing.T) {
	var work func(context.Context) error
	var done func(error)
	p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "entry", Path: vault.Path{"entry"}, Kind: vault.EntryNode}}}}
	v := New(Dependencies{Service: service.Service{Provider: p}, RunOperation: func(w func(context.Context) error, d func(error)) { work, done = w, d }})
	v.Refresh()
	if !v.loading || work == nil {
		t.Fatal("load did not yield to event loop")
	}
	err := work(context.Background())
	done(err)
	if v.loading || len(v.vaultNodes) != 1 {
		t.Fatal("completion did not update browser")
	}
}

func TestSaveYieldsAndRejectsDuplicateSubmission(t *testing.T) {
	var work func(context.Context) error
	var done func(error)
	calls := 0
	p := &memoryVault{}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}, Password: func() string { return "fixture" }, RunOperation: func(w func(context.Context) error, d func(error)) { calls++; work, done = w, d }})
	s.Open()
	done(work(context.Background()))
	calls = 0
	s.storePath = "entry"
	storeKey(s, tcell.KeyEnter, 0)
	storeKey(s, tcell.KeyEnter, 0)
	if calls != 1 || !s.saving {
		t.Fatal("submission blocked or duplicated")
	}
	done(work(context.Background()))
	storeKey(s, tcell.KeyEnter, 0)
	done(work(context.Background()))
	if s.IsOpen() || p.store.Password != "fixture" {
		t.Fatal("save completion failed")
	}
}

func TestAddLoadingBlocksTypingAndCanBeCanceled(t *testing.T) {
	var work func(context.Context) error
	var done func(error)
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: &memoryVault{}}, RunOperation: func(w func(context.Context) error, d func(error)) { work, done = w, d }})
	s.Open()
	storeKey(s, tcell.KeyRune, 'x')
	if !s.loadingEntries || s.storePath != "" {
		t.Fatal("typing accepted before entries loaded")
	}
	done(work(context.Background()))
	storeKey(s, tcell.KeyRune, 'x')
	if s.loadingEntries || s.storePath != "x" {
		t.Fatal("typing not enabled after loading")
	}
	s.Close()
	s.Open()
	storeKey(s, tcell.KeyEscape, 0)
	if s.IsOpen() {
		t.Fatal("loading form could not be canceled")
	}
	done(work(context.Background()))
	if s.IsOpen() || len(s.storeNodes) != 0 {
		t.Fatal("canceled load changed closed form")
	}
}

func TestParentSelectionRestoredAfterAsyncLoad(t *testing.T) {
	var work func(context.Context) error
	var done func(error)
	p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "alpha", Path: vault.Path{"alpha"}, Kind: vault.FolderNode}, {Name: "hello", Path: vault.Path{"hello"}, Kind: vault.FolderNode}}}}
	v := New(Dependencies{Service: service.Service{Provider: p}, RunOperation: func(w func(context.Context) error, d func(error)) { work, done = w, d }})
	v.vaultPath = vault.Path{"hello"}
	v.vaultParent()
	if !v.loading {
		t.Fatal("parent load did not yield")
	}
	done(work(context.Background()))
	if v.vaultSelected != 1 {
		t.Fatal("parent selection restored before listing completed")
	}
}

func TestClosedEditorDiscardsAsyncCompletion(t *testing.T) {
	var work func(context.Context) error
	var done func(error)
	p := &memoryVault{read: map[string]vault.Entry{"entry": {Path: vault.Path{"entry"}, Password: "fixture"}}}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}, RunOperation: func(w func(context.Context) error, d func(error)) { work, done = w, d }})
	s.OpenEdit(vault.Path{"entry"})
	s.Close()
	done(work(context.Background()))
	if s.IsOpen() || s.value != "" || s.editEntry != nil {
		t.Fatal("stale completion restored decrypted entry")
	}
}
