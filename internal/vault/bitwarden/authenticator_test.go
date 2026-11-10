package bitwarden

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"lazypass/internal/vault"
)

type organizationCodeRunner struct {
	organizationRunner
	denied bool
	codes  int
}

type organizationEditRunner struct {
	organizationRunner
	viewPassword *bool
	edit         *bool
	missingSeed  bool
}

func (r *organizationEditRunner) Run(ctx context.Context, args []string, input io.Reader, env []string) ([]byte, error) {
	output, err := r.organizationRunner.Run(ctx, args, input, env)
	if err == nil && strings.Join(args, " ") == "get item item-id" {
		var raw map[string]any
		if json.Unmarshal(output, &raw) != nil {
			return nil, errors.New("invalid fixture")
		}
		raw["viewPassword"] = r.viewPassword
		raw["edit"] = r.edit
		if !r.missingSeed {
			raw["login"].(map[string]any)["totp"] = "existing-fixture"
		}
		return json.Marshal(raw)
	}
	return output, err
}

func TestOrganizationAuthenticatorEditingAndExplicitDenials(t *testing.T) {
	allowed, denied := true, false
	for _, tc := range []struct {
		name                 string
		view, edit           *bool
		missing, unavailable bool
	}{
		{name: "CLI omits permissions"},
		{name: "explicitly allowed", view: &allowed, edit: &allowed},
		{name: "empty authenticator", missing: true},
		{name: "empty authenticator with view permission", missing: true, view: &allowed},
		{name: "hidden password", view: &denied, unavailable: true},
		{name: "read only", edit: &denied, unavailable: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &organizationEditRunner{viewPassword: tc.view, edit: tc.edit, missingSeed: tc.missing}
			p := New(t.TempDir(), "", func() (string, error) { return "fixture", nil }, r)
			ctx := context.Background()
			entry, err := p.Read(ctx, vault.Path{"Organizations", "org-id", "group-a", "item-id"})
			if err != nil || entry.AuthenticatorUnavailable != tc.unavailable {
				t.Fatal("incorrect organization edit availability")
			}
			if tc.unavailable && entry.Authenticator != "" {
				t.Fatal("explicitly restricted seed exposed")
			}
			entry.Authenticator = "replacement-fixture"
			entry.AuthenticatorChanged = true
			err = p.Write(ctx, entry)
			if tc.unavailable {
				if err == nil || r.edited != nil {
					t.Fatal("explicit denial bypassed")
				}
			} else if err != nil || r.edited["login"].(map[string]any)["totp"] != "replacement-fixture" {
				t.Fatal("organization authenticator edit failed")
			}
		})
	}
}

func (r *organizationCodeRunner) Run(ctx context.Context, args []string, input io.Reader, env []string) ([]byte, error) {
	if strings.Join(args, " ") == "get totp item-id" {
		r.codes++
		if r.denied {
			return []byte("Premium status is required to use this feature."), errors.New("exit status 1")
		}
		return []byte("123456\n"), nil
	}
	return r.organizationRunner.Run(ctx, args, input, env)
}

func TestOrganizationAuthenticatorUsesNativeGenerationWithoutViewPasswordField(t *testing.T) {
	for _, denied := range []bool{false, true} {
		r := &organizationCodeRunner{denied: denied}
		p := New(t.TempDir(), "", func() (string, error) { return "fixture", nil }, r)
		code, err := p.AuthenticatorCode(context.Background(), vault.Path{"Organizations", "org-id", "group-a", "item-id"})
		if r.codes != 1 || r.gets != 0 {
			t.Fatal("generation must delegate without requesting the item's seed or inferred permission")
		}
		if denied {
			if err == nil || code != "" {
				t.Fatal("provider refusal was bypassed")
			}
		} else if err != nil || code != "123456" {
			t.Fatal("organization generation was incorrectly refused")
		}
	}
}

func TestAuthenticatorReadPreserveReplaceAndRemove(t *testing.T) {
	r := &fakeRunner{}
	p := New(t.TempDir(), "", nil, r)
	ctx := vault.WithPassphrasePrompt(context.Background(), func(context.Context) (string, error) { return "fixture", nil })
	entry, err := p.Read(ctx, vault.Path{"hello", "world"})
	if err != nil || entry.Authenticator != "preserved" || entry.AuthenticatorUnavailable {
		t.Fatal("native authenticator mapping failed")
	}
	code, err := p.AuthenticatorCode(ctx, entry.Path)
	if err != nil || code != "123456" {
		t.Fatal("native code command failed")
	}
	for _, replacement := range []string{"otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP", ""} {
		entry.Authenticator = replacement
		entry.AuthenticatorChanged = true
		if err := p.Write(ctx, entry); err != nil {
			t.Fatal(err)
		}
		login := r.edited["login"].(map[string]any)
		if login["totp"] != replacement || len(login["uris"].([]any)) != 2 || r.edited["fields"] == nil {
			t.Fatal("seed change damaged native item fields")
		}
	}
}
