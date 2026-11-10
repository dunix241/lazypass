package vault

import (
	"strings"
	"testing"

	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

func TestLoadingAuthenticatorRetainsEntryName(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	frame := testFrame()
	v := New(Dependencies{})
	node := vault.Node{Name: "account-name", Path: vault.Path{"account-name"}, Kind: vault.EntryNode, HasAuthenticator: true}
	y := frame.Card.Y + 2
	v.drawBrowserRow(screen, frame, frame.Card, node, y, true)
	var row strings.Builder
	for x := frame.Card.X; x < frame.Card.X+frame.Card.W; x++ {
		text, _, _ := screen.Get(x, y)
		row.WriteString(text)
	}
	if !strings.Contains(row.String(), node.Name) || !strings.Contains(row.String(), "Loading…") {
		t.Fatal("loading row must contain both entry name and indicator")
	}
}
