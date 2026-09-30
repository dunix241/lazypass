package tui

import (
	"errors"
	"testing"

	"lazypass/internal/config"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

func TestStoreFormCancelFailureAndSuccess(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{}}
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p}))
	a.screen.password = "generated"
	a.screen.openStoreForm()
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if a.screen.storeForm {
		t.Fatal("escape should cancel store form")
	}
	p.err = errors.New("write failed")
	a.screen.openStoreForm()
	a.screen.storePath = "folder/name"
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !a.screen.storeForm || a.screen.status != "Store failed" {
		t.Fatalf("form=%t status=%q", a.screen.storeForm, a.screen.status)
	}
	p.err = nil
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if p.store.Path.String() != "folder/name" || p.store.Password != "generated" || a.screen.storeForm {
		t.Fatal("generated password was not stored")
	}
}

func TestStoreActionOpensStoreForm(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: &memoryVault{}}))
	a.screen.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone), nil)
	if !a.screen.storeForm {
		t.Fatal("a should open the Add to vault form")
	}
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	wasSymbols := a.screen.cfg.Symbols
	a.screen.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone), nil)
	if a.screen.cfg.Symbols == wasSymbols {
		t.Fatal("s should toggle Symbols")
	}
}

func TestStoreFormSuggestsAndInsertsFolders(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"":         {{Name: "home", Path: vault.Path{"home"}, Kind: vault.FolderNode}, {Name: "personal", Path: vault.Path{"personal"}, Kind: vault.FolderNode}},
		"personal": {{Name: "email", Path: vault.Path{"personal", "email"}, Kind: vault.FolderNode}},
	}}
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p}))
	a.screen.openStoreForm()
	if len(a.screen.storeNodes) != 2 {
		t.Fatalf("root suggestions = %#v", a.screen.storeNodes)
	}
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModNone))
	if len(a.screen.storeNodes) != 1 || a.screen.storeNodes[0].Name != "personal" {
		t.Fatalf("prefix suggestions = %#v", a.screen.storeNodes)
	}
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyTAB, 0, tcell.ModNone))
	if a.screen.storePath != "personal/" || len(a.screen.storeNodes) != 1 || a.screen.storeNodes[0].Name != "email" {
		t.Fatalf("path=%q suggestions=%#v", a.screen.storePath, a.screen.storeNodes)
	}
}

func TestStoreFormDoesNotWarnForMissingFolder(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: &memoryVault{err: vault.ErrNotFound}}))
	a.screen.openStoreForm()
	if a.screen.storeMessage != "" {
		t.Fatalf("missing folder message = %q", a.screen.storeMessage)
	}
}

func TestStoreSuggestionQuery(t *testing.T) {
	for _, test := range []struct {
		destination string
		path        string
		prefix      string
	}{
		{"", "", ""},
		{"ho", "", "ho"},
		{"Personal/", "Personal", ""},
		{"Personal/e", "Personal", "e"},
	} {
		path, prefix, err := storeSuggestionQuery(test.destination)
		if err != nil || path.String() != test.path || prefix != test.prefix {
			t.Fatalf("query(%q) = (%q, %q, %v)", test.destination, path, prefix, err)
		}
	}
}

func TestInputWordControls(t *testing.T) {
	if got := deletePreviousWord("Personal/Email"); got != "Personal/" {
		t.Fatalf("path Ctrl-W = %q", got)
	}
	if got := deletePreviousWord("two words"); got != "two" {
		t.Fatalf("text Ctrl-W = %q", got)
	}
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: &memoryVault{}}))
	a.screen.openStoreForm()
	a.screen.storePath = "Personal/Email"
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone))
	if a.screen.storePath != "Personal/" {
		t.Fatalf("store Ctrl-W = %q", a.screen.storePath)
	}
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModNone))
	if a.screen.storePath != "" {
		t.Fatalf("store Ctrl-U = %q", a.screen.storePath)
	}
	a.screen.vaultFiltering, a.screen.vaultFilter = true, "github"
	a.screen.handleVaultFilterInput(tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone))
	if a.screen.vaultFilter != "" {
		t.Fatalf("filter Ctrl-W = %q", a.screen.vaultFilter)
	}
}

func TestStoreFormHelpFitsAvailableWidth(t *testing.T) {
	for _, width := range []int{48, 38, 28, 18} {
		if got := storeFormHelp(width); uniseg.StringWidth(got) > width {
			t.Fatalf("help %q exceeds width %d", got, width)
		}
	}
}

func TestStorePickerMouseSelectsAndCompletesFolder(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"": {{Name: "home", Path: vault.Path{"home"}, Kind: vault.FolderNode}, {Name: "personal", Path: vault.Path{"personal"}, Kind: vault.FolderNode}},
	}}
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p}))
	a.screen.SetRect(0, 0, 80, 24)
	a.screen.openStoreForm()
	r, _ := a.screen.storeFormRect(0, 0, 80, 24)
	event := tcell.NewEventMouse(r.x+2, r.y+7, tcell.Button1, tcell.ModNone)
	a.screen.handleStoreMouse(tview.MouseLeftDown, event)
	if a.screen.storeSelected != 1 {
		t.Fatalf("selected = %d", a.screen.storeSelected)
	}
	a.screen.handleStoreMouse(tview.MouseLeftClick, event)
	if a.screen.storePath != "personal/" {
		t.Fatalf("path = %q", a.screen.storePath)
	}
}
