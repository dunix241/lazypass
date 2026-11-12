package vault

import (
	"context"
	"errors"
	"github.com/gdamore/tcell/v2"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
	"testing"
)

type transferProvider struct {
	memoryVault
	conflict    string
	fail        string
	transferred []string
	move        bool
}

func TestCanceledPastePreflightCannotStartMutation(t *testing.T) {
	nodes := []vault.Node{{Name: "fixture", Path: vault.Path{"fixture"}, Kind: vault.EntryNode}}
	p := &transferProvider{memoryVault: memoryVault{nodes: map[string][]vault.Node{"": nodes}}}
	var work func(context.Context) error
	var done func(error)
	v := New(Dependencies{Service: service.Service{Provider: p}, RunOperation: func(fn func(context.Context) error, finish func(error)) { work, done = fn, finish }})
	v.vaultNodes = nodes
	v.stageTransfer(false)
	v.pasteEntries()
	v.CancelTransfer()
	if err := work(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("preflight ignored cancellation: %v", err)
	}
	done(nil)
	if len(p.transferred) != 0 || v.transfer.busy || v.transfer.conflict {
		t.Fatal("late preflight started mutation or reopened dialog")
	}
}

func TestCanceledCutRecordsSuccessWithoutLateNotification(t *testing.T) {
	nodes := []vault.Node{{Name: "fixture", Path: vault.Path{"fixture"}, Kind: vault.EntryNode}}
	p := &transferProvider{memoryVault: memoryVault{nodes: map[string][]vault.Node{"": nodes}}}
	var finish func(error)
	notifications := 0
	v := New(Dependencies{Service: service.Service{Provider: p}, Notify: func(shared.Level, string) { notifications++ }})
	v.transfer = transferState{nodes: nodes, names: []string{"fixture"}, move: true}
	v.deps.RunOperation = func(work func(context.Context) error, done func(error)) {
		if err := work(context.Background()); err != nil {
			t.Fatal(err)
		}
		finish = done
	}
	v.executePaste()
	v.CancelTransfer()
	before := notifications
	finish(nil)
	if len(p.transferred) != 1 || v.HasStagedTransfers() || v.transfer.busy || notifications != before {
		t.Fatal("late cut completion lost success accounting or leaked UI updates")
	}
}

type folderTransferProvider struct{ transferProvider }

func (p *folderTransferProvider) CheckFolderTransfer(ctx context.Context, source, destination vault.Path, name string, move bool) error {
	return p.CheckTransfer(ctx, source, destination, name, move)
}
func (p *folderTransferProvider) TransferFolder(ctx context.Context, source, destination vault.Path, name string, move bool) error {
	return p.Transfer(ctx, source, destination, name, move)
}

func TestFolderMarksStagingAndDescendantDeduplication(t *testing.T) {
	nodes := []vault.Node{{Name: "folder", Path: vault.Path{"folder"}, Kind: vault.FolderNode}, {Name: "child", Path: vault.Path{"folder", "child"}, Kind: vault.EntryNode}}
	p := &folderTransferProvider{transferProvider: transferProvider{memoryVault: memoryVault{nodes: map[string][]vault.Node{"": nodes}}}}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.vaultNodes = nodes
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'm', 0))
	v.vaultSelected = 1
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'M', 0))
	if len(v.marks) != 2 {
		t.Fatal("range did not mark folder and entry")
	}
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'y', 0))
	if len(v.transfer.nodes) != 1 || v.transfer.nodes[0].Kind != vault.FolderNode {
		t.Fatal("folder descendants staged twice")
	}
	if !v.stagedEntry(nodes[0]) {
		t.Fatal("folder staging badge missing")
	}
	v.ClearMarks()
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'p', 0))
	if len(p.transferred) != 1 || p.transferred[0] != "folder" {
		t.Fatal("folder transfer not dispatched")
	}
}

func (p *transferProvider) CheckTransfer(_ context.Context, source, destination vault.Path, name string, move bool) error {
	if name == p.conflict {
		return vault.ErrConflict
	}
	return nil
}
func (p *transferProvider) Transfer(_ context.Context, source, destination vault.Path, name string, move bool) error {
	if name == p.fail {
		return errors.New("fixture failure")
	}
	p.transferred = append(p.transferred, name)
	p.move = move
	return nil
}

func TestStagingTogglesEntriesAndSwitchesMode(t *testing.T) {
	for _, key := range []rune{'y', 'x'} {
		t.Run(string(key), func(t *testing.T) {
			nodes := []vault.Node{{Name: "one", Path: vault.Path{"one"}, Kind: vault.EntryNode}, {Name: "two", Path: vault.Path{"two"}, Kind: vault.EntryNode}}
			p := &transferProvider{memoryVault: memoryVault{nodes: map[string][]vault.Node{"": nodes}}}
			v := New(Dependencies{Service: service.Service{Provider: p}})
			v.vaultNodes = nodes
			press := func(key rune) { v.HandleKey(tcell.NewEventKey(tcell.KeyRune, key, 0)) }
			press(key)
			v.vaultSelected = 1
			press(key)
			if len(v.transfer.nodes) != 2 {
				t.Fatal("staging another entry discarded previous entry")
			}
			press(key)
			if len(v.transfer.nodes) != 1 || v.transfer.nodes[0].Name != "one" {
				t.Fatal("second press did not toggle selected entry off")
			}
			v.vaultSelected = 0
			press(key)
			if len(v.transfer.nodes) != 0 {
				t.Fatal("last entry did not toggle off")
			}
			for _, action := range v.FooterActions().Actions {
				if action.Hint == "p" {
					t.Fatal("paste shown with empty clipboard")
				}
			}
			v.marks = map[string]bool{"one": true, "two": true}
			press(key)
			press(key)
			if len(v.transfer.nodes) != 0 {
				t.Fatal("marked batch did not toggle off")
			}
			press('y')
			press('x')
			if len(v.transfer.nodes) != 2 || !v.transfer.move {
				t.Fatal("switching mode deselected entries")
			}
			v.HandleKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
			if v.HasMarks() || v.HasStagedTransfers() {
				t.Fatal("Escape did not clear marks and staged entries together")
			}
		})
	}
}

func TestPastePreflightsBatchAndResolvesConflict(t *testing.T) {
	nodes := []vault.Node{{Name: "one", Path: vault.Path{"one"}, Kind: vault.EntryNode}, {Name: "two", Path: vault.Path{"two"}, Kind: vault.EntryNode}}
	p := &transferProvider{conflict: "two", memoryVault: memoryVault{nodes: map[string][]vault.Node{"": nodes}}}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.vaultNodes = nodes
	v.marks = map[string]bool{"one": true, "two": true}
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'y', 0))
	v.ClearMarks()
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'p', 0))
	if !v.transfer.conflict || len(p.transferred) != 0 {
		t.Fatal("batch wrote before resolving all conflicts")
	}
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 's', 0))
	if len(p.transferred) != 1 || p.transferred[0] != "one" || len(v.transfer.nodes) != 2 {
		t.Fatal("copy clipboard or skip semantics changed")
	}
}

func TestCutRetainsUnfinishedEntriesAndBlocksUncertainRetry(t *testing.T) {
	nodes := []vault.Node{{Name: "one", Path: vault.Path{"one"}, Kind: vault.EntryNode}, {Name: "two", Path: vault.Path{"two"}, Kind: vault.EntryNode}}
	p := &transferProvider{fail: "two", memoryVault: memoryVault{nodes: map[string][]vault.Node{"": nodes}}}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.vaultNodes = nodes
	v.marks = map[string]bool{"one": true, "two": true}
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'x', 0))
	if len(p.transferred) != 0 {
		t.Fatal("cut mutated source before paste")
	}
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'p', 0))
	if !p.move || len(v.transfer.nodes) != 1 || v.transfer.nodes[0].Name != "two" || !v.transfer.blocked {
		t.Fatal("partial cut lost unfinished state")
	}
	v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'p', 0))
	if len(p.transferred) != 1 {
		t.Fatal("uncertain mutation retried")
	}
}
