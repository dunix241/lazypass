package bitwarden

import (
	"context"
	"encoding/json"
	"strings"

	"lazypass/internal/vault"
)

// Kept separate from browser items so snapshots cannot retain authenticator seeds.
type loginAccess struct {
	Edit         *bool `json:"edit"`
	ViewPassword *bool `json:"viewPassword"`
	Login        struct {
		TOTP *string `json:"totp"`
	} `json:"login"`
}

func (a loginAccess) unavailable() bool {
	return denied(a.Edit) || denied(a.ViewPassword)
}

func denied(permission *bool) bool {
	return permission != nil && !*permission
}

func patchAuthenticator(raw map[string]json.RawMessage, login map[string]json.RawMessage, entry vault.Entry) error {
	if !entry.AuthenticatorChanged {
		return nil
	}
	var access loginAccess
	if value, ok := raw["edit"]; ok && json.Unmarshal(value, &access.Edit) != nil {
		return vault.ErrMalformed
	}
	if value, ok := raw["viewPassword"]; ok && json.Unmarshal(value, &access.ViewPassword) != nil {
		return vault.ErrMalformed
	}
	if entry.AuthenticatorUnavailable || access.unavailable() {
		return permissionDenied("edit authenticator")
	}
	login["totp"], _ = json.Marshal(entry.Authenticator)
	return nil
}

// AuthenticatorCode delegates entitlement and item restrictions to the CLI.
func (p *Provider) AuthenticatorCode(ctx context.Context, path vault.Path) (string, error) {
	if err := validPath(path); err != nil {
		return "", err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	s, err := p.snapshot(ctx)
	if err != nil {
		return "", err
	}
	item, err := s.resolve(path)
	if err != nil {
		return "", err
	}
	if item.Type != 1 {
		return "", vault.ErrUnsupported
	}
	output, err := p.run(ctx, []string{"get", "totp", item.ID}, nil)
	defer clear(output)
	if err != nil {
		return "", err
	}
	code := strings.TrimSpace(string(output))
	if len(code) != 6 && len(code) != 8 {
		return "", vault.ErrUnsupported
	}
	for _, digit := range code {
		if digit < '0' || digit > '9' {
			return "", vault.ErrMalformed
		}
	}
	return code, nil
}
