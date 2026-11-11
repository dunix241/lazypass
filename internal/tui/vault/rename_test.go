package vault

import (
	"context"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
	"testing"
)

type renameProvider struct {
	memoryVault
	name string
	path vault.Path
}

func TestRenamePopupMouseActions(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		p := &renameProvider{memoryVault: memoryVault{nodes: map[string][]vault.Node{"": {{Name: "original", Path: vault.Path{"original"}, Kind: vault.EntryNode}}}}}
		v := New(Dependencies{Service: service.Service{Provider: p}})
		v.Refresh()
		v.openRename()
		key(v, tcell.KeyCtrlU, 0)
		v.PasteTransferName("new-name")
		frame := testFrame()
		r := shared.CenteredPanel(frame.Viewport, min(64, max(0, frame.Viewport.W-2)), 7)
		bounds := shared.Rect{X: r.X + 2, Y: r.Y + r.H - 2, W: r.W - 4, H: 1}
		hint := "<CR>"
		if cancel {
			hint = "<esc>"
		}
		for x := bounds.X; x < bounds.X+bounds.W; x++ {
			item, ok := (shared.Footer{Actions: v.renameActions()}).HitAction(bounds, shared.FooterOptions{MaxRows: 1}, x, bounds.Y)
			if ok && item.Hint == hint {
				v.HandleMouse(tview.MouseLeftClick, tcell.NewEventMouse(x, bounds.Y, tcell.Button1, 0), frame)
				break
			}
		}
		if v.renameTarget != nil || (p.name == "new-name") == cancel {
			t.Fatal("rename popup click failed")
		}
	}
}

func TestCanceledRenameDiscardsLateSuccess(t *testing.T) {
	p := &renameProvider{memoryVault: memoryVault{nodes: map[string][]vault.Node{"": {{Name: "original", Path: vault.Path{"original"}, Kind: vault.EntryNode}}}}}
	var finish func(error)
	notifications := 0
	v := New(Dependencies{Service: service.Service{Provider: p}, Notify: func(shared.Level, string) { notifications++ }})
	v.Refresh()
	v.deps.RunOperation = func(_ func(context.Context) error, done func(error)) { finish = done }
	key(v, tcell.KeyRune, 'r')
	key(v, tcell.KeyCtrlU, 0)
	v.PasteTransferName("renamed")
	key(v, tcell.KeyEnter, 0)
	v.CancelTransfer()
	before := notifications
	finish(nil)
	if notifications != before || v.transfer.busy || !v.transfer.blocked {
		t.Fatal("canceled rename applied late success")
	}
}

func (p *renameProvider) Rename(_ context.Context, node vault.Node, name string) error {
	p.name = name
	p.path = node.Path
	return nil
}

func TestRenamePrefillPasteCancelAndSubmit(t *testing.T) {
	p := &renameProvider{memoryVault: memoryVault{nodes: map[string][]vault.Node{"": {{Name: "original", Path: vault.Path{"original"}, Kind: vault.EntryNode}}}}}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.Refresh()
	key(v, tcell.KeyRune, 'r')
	if v.renameTarget == nil || v.transfer.editor.Value() != "original" {
		t.Fatal("rename not prefilled")
	}
	key(v, tcell.KeyEscape, 0)
	if p.name != "" || v.IsTransferActive() {
		t.Fatal("cancel renamed entry")
	}
	key(v, tcell.KeyRune, 'r')
	key(v, tcell.KeyCtrlU, 0)
	v.PasteTransferName("renamed")
	key(v, tcell.KeyEnter, 0)
	if p.name != "renamed" || p.path.String() != "original" {
		t.Fatal("rename did not submit frozen source")
	}
}
