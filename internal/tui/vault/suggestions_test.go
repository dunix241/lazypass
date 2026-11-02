package vault

import (
	"fmt"
	"testing"

	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestAddSuggestionsIncludeExistingEntries(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"":      {{Name: "hello", Path: vault.Path{"hello"}, Kind: vault.FolderNode}},
		"hello": {{Name: "world", Path: vault.Path{"hello", "world"}, Kind: vault.EntryNode}},
	}}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	s.Open()
	storeKey(s, tcell.KeyTAB, 0)
	if s.storePath != "hello/" || len(s.storeNodes) != 1 || s.storeNodes[0].Kind != vault.EntryNode {
		t.Fatal("existing entry missing from destination picker")
	}
	storeKey(s, tcell.KeyTAB, 0)
	if s.storePath != "hello/" {
		t.Fatal("entry completed as a folder")
	}
	storeKey(s, tcell.KeyRune, 'w')
	if len(s.storeNodes) != 1 || s.storeNodes[0].Name != "world" {
		t.Fatal("entry prefix filtering failed")
	}
}

func TestSuggestionMouseMatchesScrolledRows(t *testing.T) {
	s := NewStoreForm(StoreDependencies{})
	for i := range 8 {
		name := fmt.Sprintf("folder-%d", i)
		s.storeNodes = append(s.storeNodes, vault.Node{Name: name, Path: vault.Path{name}, Kind: vault.FolderNode})
	}
	s.storeSelected = 7
	f := testFrame()
	f.Viewport.W, f.Viewport.H = 80, 24
	r, visible := s.storeFormRect(f.Viewport)
	start, _ := s.suggestionWindow(visible)
	s.HandleMouse(tview.MouseLeftDown, tcell.NewEventMouse(r.X+2, r.Y+5, tcell.Button1, tcell.ModNone), f)
	if s.storeSelected != start {
		t.Fatal("mouse selection did not match displayed row")
	}
	s.storeMessage = "Items unavailable"
	before := s.storeSelected
	s.HandleMouse(tview.MouseLeftDown, tcell.NewEventMouse(r.X+2, r.Y+6, tcell.Button1, tcell.ModNone), f)
	if s.storeSelected != before {
		t.Fatal("mouse selected a hidden suggestion")
	}
}

func TestSuggestionWindowBounds(t *testing.T) {
	s := NewStoreForm(StoreDependencies{})
	s.storeNodes = make([]vault.Node, 6)
	for _, selected := range []int{-1, 0, 5, 20} {
		s.storeSelected = selected
		for _, visible := range []int{0, 1, 4, 10} {
			start, end := s.suggestionWindow(visible)
			if start < 0 || end < start || end > len(s.storeNodes) || end-start > visible {
				t.Fatalf("invalid window %d:%d", start, end)
			}
		}
	}
	s.storeSelected = 20
	s.insertStoreSuggestion()
}
