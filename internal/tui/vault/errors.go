package vault

import (
	"errors"

	"lazypass/internal/debug"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

func copyErrorMessage(err error) string {
	var diagnostic vault.Diagnostic
	if errors.As(err, &diagnostic) {
		return diagnostic.UserMessage()
	}
	switch {
	case errors.Is(err, vault.ErrLocked):
		return "Vault locked (no details)"
	case errors.Is(err, vault.ErrNotFound):
		return "Vault entry not found"
	case errors.Is(err, vault.ErrMalformed):
		return "Vault entry format invalid"
	case errors.Is(err, vault.ErrUninitialized):
		return "Vault not initialized"
	case errors.Is(err, vault.ErrUnavailable):
		return "Vault unavailable"
	case errors.Is(err, service.ErrClipboard):
		return "Clipboard unavailable"
	default:
		return "Copy failed"
	}
}

func vaultFailureCause(err error) debug.Cause {
	switch {
	case errors.Is(err, vault.ErrLocked):
		return debug.Locked
	case errors.Is(err, vault.ErrNotFound):
		return debug.NotFound
	case errors.Is(err, vault.ErrUninitialized), errors.Is(err, vault.ErrInvalidPath), errors.Is(err, vault.ErrMalformed):
		return debug.Invalid
	case errors.Is(err, vault.ErrUnavailable):
		return debug.Unavailable
	case errors.Is(err, vault.ErrConflict):
		return debug.Conflict
	case errors.Is(err, service.ErrClipboard):
		return debug.Clipboard
	default:
		return debug.Unknown
	}
}

func storeErrorMessage(err error) string {
	var diagnostic vault.Diagnostic
	if errors.As(err, &diagnostic) {
		return diagnostic.UserMessage()
	}
	switch {
	case errors.Is(err, vault.ErrUnsupported):
		return "This vault does not support empty folders"
	case errors.Is(err, vault.ErrConflict):
		return "Destination already exists"
	case errors.Is(err, vault.ErrInvalidPath):
		return "Invalid vault destination"
	case errors.Is(err, vault.ErrUninitialized):
		return "Vault is not initialized. Run: pass init <recipient>"
	case errors.Is(err, vault.ErrLocked):
		return "Vault locked (no details)"
	case errors.Is(err, vault.ErrUnavailable):
		return "Vault is unavailable. Check pass configuration."
	default:
		return "Store failed"
	}
}
