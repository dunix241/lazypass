package vault

import (
	"context"
	"errors"
	"testing"

	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
)

type deletingVault struct {
	memoryVault
	deleted vault.Path
	folder  bool
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
		key(v, tcell.KeyRune, 'n')
		if v.IsConfirmingDelete() || p.deleted != nil {
			t.Fatal("cancel deleted target")
		}
		key(v, tcell.KeyRune, 'd')
		key(v, tcell.KeyRune, 'y')
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
