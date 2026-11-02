package vault

import (
	"errors"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
	"testing"
)

func storeKey(s *StoreForm, k tcell.Key, r rune) { s.HandleKey(tcell.NewEventKey(k, r, tcell.ModNone)) }

func TestStoreFormCancelFailureAndSuccess(t *testing.T) {
	p := &memoryVault{}
	var message string
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}, Password: func() string { return "generated" }, Notify: func(_ shared.Level, m string) { message = m }})
	s.Open()
	storeKey(s, tcell.KeyEscape, 0)
	if s.IsOpen() {
		t.Fatal("Escape did not close")
	}
	p.err = errors.New("write failed")
	s.Open()
	s.storePath = "folder/name"
	storeKey(s, tcell.KeyEnter, 0)
	if !s.IsOpen() || message != "Store failed" {
		t.Fatal("failure did not retain form")
	}
	p.err = nil
	storeKey(s, tcell.KeyEnter, 0)
	storeKey(s, tcell.KeyEnter, 0)
	if p.store.Path.String() != "folder/name" || p.store.Password != "generated" || s.IsOpen() || s.storePath != "" || len(s.storeNodes) != 0 {
		t.Fatal("success/cleanup failed")
	}
}

func TestStoreFormSuggestsAndInsertsFolders(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "home", Path: vault.Path{"home"}, Kind: vault.FolderNode}, {Name: "personal", Path: vault.Path{"personal"}, Kind: vault.FolderNode}}, "personal": {{Name: "email", Path: vault.Path{"personal", "email"}, Kind: vault.FolderNode}}}}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	s.Open()
	if len(s.storeNodes) != 2 {
		t.Fatal("root suggestions missing")
	}
	storeKey(s, tcell.KeyRune, 'p')
	if len(s.storeNodes) != 1 || s.storeNodes[0].Name != "personal" {
		t.Fatal("prefix filtering failed")
	}
	storeKey(s, tcell.KeyTAB, 0)
	if s.storePath != "personal/" || len(s.storeNodes) != 1 || s.storeNodes[0].Name != "email" {
		t.Fatal("Tab completion failed")
	}
	s.storePath = "Personal/Email"
	storeKey(s, tcell.KeyCtrlW, 0)
	if s.storePath != "Personal/" {
		t.Fatal("Ctrl-W failed")
	}
	storeKey(s, tcell.KeyCtrlU, 0)
	if s.storePath != "" {
		t.Fatal("Ctrl-U failed")
	}
}

func TestStoreFormDoesNotWarnForMissingFolder(t *testing.T) {
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: &memoryVault{err: vault.ErrNotFound}}})
	s.Open()
	if s.storeMessage != "" {
		t.Fatal("missing folder warning")
	}
}

func TestStoreSuggestionQuery(t *testing.T) {
	for _, test := range []struct{ destination, path, prefix string }{{"", "", ""}, {"ho", "", "ho"}, {"Personal/", "Personal", ""}, {"Personal/e", "Personal", "e"}} {
		path, prefix, err := storeSuggestionQuery(test.destination)
		if err != nil || path.String() != test.path || prefix != test.prefix {
			t.Fatal("query parse failed")
		}
	}
	if shared.DeletePreviousWord("two words") != "two" || shared.DeletePreviousWord("Personal/Email") != "Personal/" {
		t.Fatal("word deletion changed")
	}
}

func TestStoreFormHelpFitsAvailableWidth(t *testing.T) {
	for _, width := range []int{48, 38, 28, 18} {
		if uniseg.StringWidth(storeFormHelp(width)) > width {
			t.Fatal("help overflow")
		}
	}
}

func TestStorePickerMouseSelectsAndCompletesFolder(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "home", Path: vault.Path{"home"}, Kind: vault.FolderNode}, {Name: "personal", Path: vault.Path{"personal"}, Kind: vault.FolderNode}}}}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	s.Open()
	f := testFrame()
	r, _ := s.storeFormRect(f.Viewport)
	event := tcell.NewEventMouse(r.X+2, r.Y+6, tcell.Button1, tcell.ModNone)
	s.HandleMouse(tview.MouseLeftDown, event, f)
	if s.storeSelected != 1 {
		t.Fatal("mouse selection failed")
	}
	s.HandleMouse(tview.MouseLeftClick, event, f)
	if s.storePath != "personal/" {
		t.Fatal("mouse completion failed")
	}
}
