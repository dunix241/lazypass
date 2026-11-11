package vault

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

type markDeleteProvider struct {
	memoryVault
	deleted     []string
	fail        string
	afterDelete func()
}

func (p *markDeleteProvider) Capabilities() vault.Capabilities {
	return vault.Capabilities{Delete: true}
}
func (p *markDeleteProvider) Delete(_ context.Context, path vault.Path) error {
	if path.String() == p.fail {
		return errors.New("fixture failure")
	}
	p.deleted = append(p.deleted, path.String())
	if p.afterDelete != nil {
		p.afterDelete()
	}
	return nil
}

func TestMarkedDeletionCancellationStopsBetweenEntries(t *testing.T) {
	nodes := []vault.Node{
		{Name: "a", Path: vault.Path{"a"}, Kind: vault.EntryNode},
		{Name: "b", Path: vault.Path{"b"}, Kind: vault.EntryNode},
	}
	p := &markDeleteProvider{memoryVault: memoryVault{nodes: map[string][]vault.Node{"": nodes}}}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.vaultNodes = nodes
	v.marks = map[string]bool{"a": true, "b": true}
	p.afterDelete = func() { v.CancelMarkedDelete() }
	v.deleteMarked(nodes)
	if len(p.deleted) != 1 || v.marks["a"] || !v.marks["b"] || v.deleting || v.markedDeleteCancel != nil {
		t.Fatal("cancellation did not preserve the unfinished marked set")
	}
}

func TestMarkIdentityDoesNotCollideForLocalReservedNames(t *testing.T) {
	first := vault.Node{Path: vault.Path{"Folders", "one", "entry"}}
	second := vault.Node{Path: vault.Path{"Folders", "two", "entry"}}
	if markID(first) == markID(second) {
		t.Fatal("local paths collapsed into provider identity")
	}
	first.Shared, second.Shared = true, true
	if markID(first) != markID(second) {
		t.Fatal("shared references not deduplicated")
	}
}

func TestMarkedDeleteConfirmationAndPartialFailure(t *testing.T) {
	nodes := []vault.Node{
		{Name: "a", Path: vault.Path{"a"}, Kind: vault.EntryNode},
		{Name: "b", Path: vault.Path{"b"}, Kind: vault.EntryNode},
	}
	p := &markDeleteProvider{fail: "b", memoryVault: memoryVault{nodes: map[string][]vault.Node{"": nodes}}}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.vaultNodes = nodes
	v.marks = map[string]bool{"a": true, "b": true}
	v.openMarkedDelete()
	if v.deleteDialog.Options.RequireText != "DELETE 2" || !strings.Contains(v.Footer(80), "confirm") {
		t.Fatal("marked confirmation hints incorrect")
	}
	v.deleteMarked(nodes)
	if len(p.deleted) != 1 || v.marks["a"] || !v.marks["b"] {
		t.Fatal("partial delete did not retain unresolved marks")
	}
}

func TestMarkRangeNearestAndToggle(t *testing.T) {
	v := New(Dependencies{})
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		v.vaultNodes = append(v.vaultNodes, vault.Node{Name: name, Path: vault.Path{name}, Kind: vault.EntryNode})
	}
	v.vaultSelected = 0
	v.toggleMark(false)
	v.vaultSelected = 4
	v.toggleMark(false)
	v.vaultSelected = 2
	v.toggleMark(true)
	if len(v.marks) != 4 || !v.marks["b"] || v.marks["d"] {
		t.Fatal("range did not choose preceding mark on tie")
	}
	v.toggleMark(false)
	if v.marks["c"] {
		t.Fatal("mark did not toggle off")
	}
}

func TestMarkedDeletionDeletesOnlyMarkedEntries(t *testing.T) {
	p := &markDeleteProvider{}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	nodes := []vault.Node{
		{Name: "a", Path: vault.Path{"a"}, Kind: vault.EntryNode},
		{Name: "b", Path: vault.Path{"b"}, Kind: vault.EntryNode},
	}
	v.vaultNodes = nodes
	v.marks = map[string]bool{"a": true, "b": true}
	v.deleteMarked(nodes)
	if len(p.deleted) != 2 || len(v.marks) != 0 {
		t.Fatal("bulk deletion incomplete")
	}
}

type retryMarkedProvider struct {
	markDeleteProvider
	requests int
}

func (p *retryMarkedProvider) Delete(ctx context.Context, path vault.Path) error {
	if path.String() == "b" {
		p.requests++
		if p.requests == 1 {
			return vault.ErrPromptRequired
		}
	}
	return p.markDeleteProvider.Delete(ctx, path)
}

func TestMarkedDeletePromptRetryResumesAfterConfirmedSuccess(t *testing.T) {
	p := &retryMarkedProvider{}
	v := New(Dependencies{
		Service: service.Service{Provider: p},
		RunOperation: func(work func(context.Context) error, done func(error)) {
			err := work(context.Background())
			if errors.Is(err, vault.ErrPromptRequired) {
				err = work(context.Background())
			}
			done(err)
		},
	})
	nodes := []vault.Node{
		{Name: "a", Path: vault.Path{"a"}, Kind: vault.EntryNode},
		{Name: "b", Path: vault.Path{"b"}, Kind: vault.EntryNode},
	}
	v.vaultNodes = nodes
	v.marks = map[string]bool{"a": true, "b": true}
	v.deleteMarked(nodes)
	if len(p.deleted) != 2 || p.deleted[0] != "a" || p.deleted[1] != "b" || v.HasMarks() {
		t.Fatal("prompt retry repeated a confirmed deletion")
	}
}

func TestMarkedDeleteConfirmationFreezesPathsAcrossRefresh(t *testing.T) {
	p := &markDeleteProvider{}
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.vaultNodes = []vault.Node{{Name: "fixture", Path: vault.Path{"original"}, Kind: vault.EntryNode}}
	v.marks = map[string]bool{"original": true}
	v.openMarkedDelete()
	// Simulate refreshed rows reusing a path buffer after confirmation opens.
	v.vaultNodes[0].Path[0] = "replacement"
	v.PasteConfirmation("DELETE 1")
	v.handleDeleteKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if len(p.deleted) != 1 || p.deleted[0] != "original" {
		t.Fatal("refreshed row redirected confirmed deletion")
	}
}
