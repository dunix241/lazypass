package bitwarden

import (
	"context"
	"encoding/json"
	"lazypass/internal/otp"
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
	reader, release, err := p.authenticatorReader(ctx)
	if err != nil {
		return "", err
	}
	defer release()
	s, err := reader.snapshot(ctx)
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
	output, err := reader.run(ctx, []string{"get", "totp", item.ID}, nil)
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

// Independent command state allows bounded read concurrency while the parent
// read lock excludes mutations and explicit locking for the command's lifetime.
func (p *Provider) authenticatorReader(ctx context.Context) (*Provider, func(), error) {
	p.mu.RLock()
	if p.metadata == nil || p.session == "" {
		p.mu.RUnlock()
		p.mu.Lock()
		_, err := p.snapshot(ctx)
		p.mu.Unlock()
		if err != nil {
			return nil, nil, err
		}
		p.mu.RLock()
	}
	reader := New(p.DataDir, p.Server, p.Prompt, p.Runner)
	reader.session = p.session
	reader.metadata = p.metadata
	return reader, p.mu.RUnlock, nil
}

func (p *Provider) AuthenticatorConcurrency() int { return 4 }

// Authorize once through native generation before retaining a session-only key.
func (p *Provider) AuthenticatorKey(ctx context.Context, path vault.Path) (otp.Key, error) {
	reader, release, err := p.authenticatorReader(ctx)
	if err != nil {
		return otp.Key{}, err
	}
	defer release()
	if _, err := reader.AuthenticatorCode(ctx, path); err != nil {
		return otp.Key{}, err
	}
	entry, err := reader.Read(ctx, path)
	if err != nil {
		return otp.Key{}, err
	}
	if entry.AuthenticatorUnavailable || entry.Authenticator == "" {
		return otp.Key{}, vault.ErrUnsupported
	}
	if err := ctx.Err(); err != nil {
		return otp.Key{}, err
	}
	return otp.Parse(entry.Authenticator)
}
