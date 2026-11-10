package vault

import (
	"context"
	"errors"
	"testing"

	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

func TestReconcileAppliesRowsBeforeSuccess(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "new", Path: vault.Path{"new"}, Kind: vault.EntryNode}}}}
	var finish func()
	var messages []string
	v := New(Dependencies{Service: service.Service{Provider: p}})
	v.deps.RunOperation = func(work func(context.Context) error, done func(error)) {
		finish = func() { done(work(context.Background())) }
	}
	v.deps.Progress = func(text string) { messages = append(messages, text) }
	v.deps.Notify = func(level shared.Level, text string) {
		if level == shared.Success && (len(v.vaultNodes) != 1 || v.vaultNodes[0].Name != "new") {
			t.Error("success preceded updated rows")
		}
		messages = append(messages, text)
	}
	v.Reconcile("Added")
	if len(messages) != 1 || messages[0] != "Refreshing…" {
		t.Fatalf("premature completion: %v", messages)
	}
	finish()
	if messages[len(messages)-1] != "Added" {
		t.Fatalf("missing completion: %v", messages)
	}
}

func TestReconcileReportsSavedButRefreshFailed(t *testing.T) {
	var done func(error)
	var level shared.Level
	var text string
	v := New(Dependencies{RunOperation: func(_ func(context.Context) error, complete func(error)) { done = complete }, Notify: func(l shared.Level, message string) { level, text = l, message }})
	v.Reconcile("Deleted")
	done(errors.New("offline"))
	if level != shared.Warning || text != "Deleted, but refresh failed; showing cached entries" {
		t.Fatalf("incorrect outcome: %v %s", level, text)
	}
}
