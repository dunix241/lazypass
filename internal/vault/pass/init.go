package pass

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"lazypass/internal/vault"
)

// Initialize delegates recipient validation and encryption setup to pass/GPG.
func (p *Provider) Initialize(ctx context.Context, recipient string) error {
	if strings.TrimSpace(recipient) == "" || strings.HasPrefix(recipient, "-") {
		return vault.ErrInvalidPath
	}
	for _, r := range recipient {
		if unicode.IsControl(r) {
			return vault.ErrInvalidPath
		}
	}
	if _, err := os.Lstat(filepath.Join(p.storeDir(), ".gpg-id")); err == nil {
		return vault.ErrConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return vault.ErrUnavailable
	}
	_, err := p.run(ctx, []string{"init", "--", recipient}, nil)
	return err
}
