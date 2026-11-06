package bitwarden

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"lazypass/internal/vault"
)

func TestServerAccessDenialAndRecovery(t *testing.T) {
	status := http.StatusForbidden
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/config" {
			t.Error("wrong probe endpoint")
		}
		w.WriteHeader(status)
	}))
	defer server.Close()
	for _, code := range []int{http.StatusForbidden, http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusBadGateway, http.StatusFound, http.StatusOK} {
		status = code
		err := checkServerAccess(context.Background(), server.Client(), server.URL)
		if (err == nil) != (code == http.StatusOK) {
			t.Fatalf("HTTP %d: %v", code, err)
		}
		if err != nil && (errors.Is(err, vault.ErrLocked) || errors.Is(err, vault.ErrUninitialized)) {
			t.Fatal("access failure misclassified as login failure")
		}
	}
}

type blockedRunner struct {
	fakeRunner
	blocked bool
}

func (r *blockedRunner) CheckServerAccess(context.Context, string) error {
	if r.blocked {
		return accessDenied()
	}
	return nil
}

func TestBlockedAccessDoesNotInvokeCLIOrClearSession(t *testing.T) {
	r := &blockedRunner{blocked: true}
	p := New(t.TempDir(), "", nil, r)
	p.session = "fixture-session"
	p.metadata = &snapshot{}
	if _, err := p.run(context.Background(), []string{"sync"}, nil); err == nil {
		t.Fatal("blocked command ran")
	}
	if r.calls != 0 || p.session != "fixture-session" || p.metadata == nil {
		t.Fatal("access denial modified session or invoked CLI")
	}
	r.blocked = false
	if _, err := p.run(context.Background(), []string{"status"}, nil); err != nil {
		t.Fatal(err)
	}
	if r.calls != 1 {
		t.Fatal("recovered connection did not retry CLI")
	}
}
