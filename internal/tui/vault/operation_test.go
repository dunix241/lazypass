package vault

import (
	"context"
	"testing"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

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
	if s.IsOpen() || p.store.Password != "fixture" {
		t.Fatal("save completion failed")
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
