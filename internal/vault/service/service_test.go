package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lazypass/internal/vault"
)

type copyProvider struct{}

func (copyProvider) Name() string                     { return "test" }
func (copyProvider) Capabilities() vault.Capabilities { return vault.Capabilities{} }
func (copyProvider) List(context.Context, vault.Path) ([]vault.Node, error) {
	return nil, nil
}
func (copyProvider) Read(context.Context, vault.Path) (vault.Entry, error) {
	return vault.Entry{Password: "secret-value"}, nil
}
func (copyProvider) Write(context.Context, vault.Entry) error { return nil }
func (copyProvider) Delete(context.Context, vault.Path) error { return nil }
func (copyProvider) Sync(context.Context) (vault.SyncResult, error) {
	return vault.SyncResult{}, nil
}

func TestCopyFailureIdentifiesClipboardWithoutExposingSecret(t *testing.T) {
	s := Service{Provider: copyProvider{}, Copy: func(string) error { return errors.New("secret-value in clipboard error") }}
	err := s.CopyPassword(context.Background(), vault.Path{"entry"})
	if !errors.Is(err, ErrClipboard) || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("copy error = %v", err)
	}
}
