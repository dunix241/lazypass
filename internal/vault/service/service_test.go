package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lazypass/internal/vault"
)

func TestCreateRequiresProviderCreateContract(t *testing.T) {
	s := Service{Provider: copyProvider{}}
	if err := s.Create(context.Background(), vault.Entry{Path: vault.Path{"entry"}, Password: "fixture"}); !errors.Is(err, vault.ErrUnsupported) {
		t.Fatalf("unsafe write fallback: %v", err)
	}
}

type copyProvider struct{}

type typedCopyProvider struct {
	copyProvider
	entry vault.Entry
}

func (p typedCopyProvider) Read(context.Context, vault.Path) (vault.Entry, error) {
	return p.entry, nil
}

func TestCopyPrimaryFieldsAndEmptyValues(t *testing.T) {
	for _, test := range []struct {
		entry vault.Entry
		want  string
		err   error
	}{
		{vault.Entry{Password: "password"}, "password", nil},
		{vault.Entry{Type: vault.CardEntry, Fields: map[string]string{"number": "1234"}}, "1234", nil},
		{vault.Entry{Type: vault.NoteEntry, Notes: "first\nsecond"}, "first\nsecond", nil},
		{vault.Entry{Type: vault.SSHKeyEntry, Fields: map[string]string{"publicKey": "public-key", "privateKey": "must-not-copy"}}, "public-key", nil},
		{vault.Entry{}, "", ErrEmptyCopy},
		{vault.Entry{Type: vault.IdentityEntry}, "", ErrIdentityCopy},
	} {
		called := false
		s := Service{Provider: typedCopyProvider{entry: test.entry}, Copy: func(value string) error {
			called = true
			if value != test.want {
				t.Fatal("wrong copy field")
			}
			return nil
		}}
		err := s.CopyPassword(context.Background(), vault.Path{"fixture"})
		if !errors.Is(err, test.err) || called != (test.err == nil) {
			t.Fatalf("copy result: %v", err)
		}
	}
}

type sshCopyProvider struct{ copyProvider }

func (sshCopyProvider) Read(context.Context, vault.Path) (vault.Entry, error) {
	return vault.Entry{Type: vault.SSHKeyEntry, Fields: map[string]string{"publicKey": "ssh-ed25519 public-fixture", "privateKey": "private-fixture"}}, nil
}

func TestSSHCopyUsesPublicKey(t *testing.T) {
	var copied string
	s := Service{Provider: sshCopyProvider{}, Copy: func(value string) error { copied = value; return nil }}
	if err := s.CopyPassword(context.Background(), vault.Path{"key"}); err != nil {
		t.Fatal(err)
	}
	if copied != "ssh-ed25519 public-fixture" {
		t.Fatal("SSH copy did not select the public key")
	}
}

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
