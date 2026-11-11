package vault

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/age"
	"lazypass/internal/vault/service"
)

func TestAgeBrowserTransferRenameAndResize(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "store")
	identity := filepath.Join(dir, "identity")
	if err := age.Initialize(store, identity, "public-fixture-passphrase"); err != nil {
		t.Fatal(err)
	}
	p := age.New(store, identity, func() (string, error) { return "public-fixture-passphrase", nil })
	defer p.Lock()
	ctx := context.Background()
	if err := p.Create(ctx, vault.Entry{Path: vault.Path{"source", "entry"}, Password: "public-fixture", Notes: "preserved notes"}); err != nil {
		t.Fatal(err)
	}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.Refresh()
	press := func(r rune) { key(v, tcell.KeyRune, r) }
	press('y')
	press('p')
	if !v.transfer.conflict {
		t.Fatal("same-folder copy did not request rename")
	}
	press('r')
	key(v, tcell.KeyCtrlU, 0)
	v.PasteTransferName("copy-🙂")
	key(v, tcell.KeyEnter, 0)
	loaded, err := p.Read(ctx, vault.Path{"copy-🙂", "entry"})
	if err != nil || loaded.Notes != "preserved notes" {
		t.Fatal("age recursive paste lost contents")
	}
	// Locate by path instead of assuming ordering after refresh.
	for i, node := range v.vaultDisplayNodes() {
		if node.Name == "source" {
			v.vaultSelected = i
		}
	}
	press('r')
	key(v, tcell.KeyCtrlU, 0)
	v.PasteTransferName("renamed-🙂")
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	for _, width := range []int{84, 40, 32} {
		sim.SetSize(width, 20)
		frame := testFrame()
		frame.Viewport = shared.Rect{W: width, H: 20}
		frame.Card = shared.Rect{X: 1, Y: 1, W: width - 2, H: 18}
		v.Draw(sim, frame)
	}
	key(v, tcell.KeyEnter, 0)
	if _, err := p.Read(ctx, vault.Path{"renamed-🙂", "entry"}); err != nil {
		t.Fatal("age folder rename failed")
	}
	press('y')
	key(v, tcell.KeyEscape, 0)
	if v.HasStagedTransfers() {
		t.Fatal("age staging not cleared")
	}
}
