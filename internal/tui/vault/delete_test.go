package vault

import (
	"context"
	"errors"
	"testing"

	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
)

type deletingVault struct {
	memoryVault
	deleted vault.Path
	folder  bool
}

func TestFolderConfirmationRejectsIncompleteTextAndUsesRedBorder(t *testing.T) {
	p := &deletingVault{memoryVault: memoryVault{nodes: map[string][]vault.Node{"": {{Name: "target", Path: vault.Path{"target"}, Kind: vault.FolderNode}}}}}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.Refresh()
	v.openDelete()
	for _, r := range "targe" {
		key(v, tcell.KeyRune, r)
	}
	key(v, tcell.KeyEnter, 0)
	if p.deleted != nil || !v.IsConfirmingDelete() {
		t.Fatal("incomplete text authorized deletion")
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, size := range [][2]int{{32, 18}, {80, 24}, {120, 40}} {
		screen.SetSize(size[0], size[1])
		frame := shared.Frame{Viewport: shared.Rect{W: size[0], H: size[1]}, Palette: shared.DefaultPalette()}
		v.drawDelete(screen, frame)
		redBorder := false
		for y := 0; y < size[1]; y++ {
			for x := 0; x < size[0]; x++ {
				text, style, _ := screen.Get(x, y)
				fg, _, _ := style.Decompose()
				if text == "╭" && fg == tcell.ColorRed {
					redBorder = true
				}
			}
		}
		if !redBorder {
			t.Fatal("destructive border is not red")
		}
	}
	key(v, tcell.KeyEscape, 0)
	if p.deleted != nil || v.IsConfirmingDelete() {
		t.Fatal("cancel invoked deletion")
	}
}

func (p *deletingVault) Capabilities() vault.Capabilities { return vault.Capabilities{Delete: true} }
func (p *deletingVault) Delete(_ context.Context, path vault.Path) error {
	p.deleted = path
	return p.err
}
func (p *deletingVault) DeleteFolder(_ context.Context, path vault.Path) error {
	p.deleted = path
	p.folder = true
	return p.err
}

func TestDeleteEntryAndFolderRequireConfirmation(t *testing.T) {
	for _, kind := range []vault.NodeKind{vault.EntryNode, vault.FolderNode} {
		p := &deletingVault{memoryVault: memoryVault{nodes: map[string][]vault.Node{"": {{Name: "target", Path: vault.Path{"target"}, Kind: kind}}}}}
		v := New(Dependencies{Service: service.Service{Provider: p}})
		v.Refresh()
		key(v, tcell.KeyRune, 'd')
		if !v.IsConfirmingDelete() || p.deleted != nil {
			t.Fatal("delete did not wait for confirmation")
		}
		key(v, tcell.KeyRune, 'v')
		if !v.IsConfirmingDelete() {
			t.Fatal("unexpected key dismissed confirmation")
		}
		key(v, tcell.KeyEscape, 0)
		if v.IsConfirmingDelete() || p.deleted != nil {
			t.Fatal("cancel deleted target")
		}
		key(v, tcell.KeyRune, 'd')
		if kind == vault.FolderNode {
			key(v, tcell.KeyEscape, 0)
			key(v, tcell.KeyRune, 'D')
			for _, r := range "target" {
				key(v, tcell.KeyRune, r)
			}
			key(v, tcell.KeyEnter, 0)
		} else {
			key(v, tcell.KeyRune, 'y')
		}
		if v.IsConfirmingDelete() || p.deleted.String() != "target" || p.folder != (kind == vault.FolderNode) {
			t.Fatal("confirmed deletion failed")
		}
	}
}

func TestDeleteParentAndFailure(t *testing.T) {
	p := &deletingVault{}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.vaultPath = vault.Path{"folder"}
	v.Refresh()
	key(v, tcell.KeyRune, 'd')
	if v.IsConfirmingDelete() {
		t.Fatal("parent row deletable")
	}
	v.vaultPath = nil
	v.vaultNodes = []vault.Node{{Name: "entry", Path: vault.Path{"entry"}, Kind: vault.EntryNode}}
	key(v, tcell.KeyRune, 'd')
	p.err = errors.New("secret external error")
	key(v, tcell.KeyRune, 'y')
	if v.IsConfirmingDelete() {
		t.Fatal("failed delete retained confirmation")
	}
}

func TestAsyncDeletionSuppressesRepeatAndAllowsCancel(t *testing.T) {
	var work func(context.Context) error
	var done func(error)
	p := &deletingVault{memoryVault: memoryVault{nodes: map[string][]vault.Node{"": {{Name: "target", Path: vault.Path{"target"}, Kind: vault.EntryNode}}}}}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.Refresh()
	key(v, tcell.KeyRune, 'd')
	key(v, tcell.KeyEscape, 0)
	if v.deleting {
		t.Fatal("cancel marked deletion in progress")
	}
	v.deps.RunOperation = func(w func(context.Context) error, d func(error)) { work, done = w, d }
	key(v, tcell.KeyRune, 'd')
	key(v, tcell.KeyRune, 'y')
	key(v, tcell.KeyRune, 'd')
	if !v.deleting || v.IsConfirmingDelete() {
		t.Fatal("repeat deletion accepted")
	}
	err := work(context.Background())
	v.deps.RunOperation = nil
	done(err)
	if v.deleting {
		t.Fatal("deletion busy state retained")
	}
}
