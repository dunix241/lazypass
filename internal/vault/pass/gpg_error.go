package pass

import (
	"fmt"
	"strings"

	"lazypass/internal/debug"
	"lazypass/internal/vault"
)

type GPGReason uint8

const (
	gpgUnknown GPGReason = iota
	gpgNoSecretKey
	gpgNoPinentry
	gpgNoTerminal
	gpgAgentUnavailable
	gpgBadPassphrase
	gpgCancelled
	gpgDecryptionFailed
)

type GPGFailure struct {
	Reason   GPGReason
	ExitCode int
}

func (e *GPGFailure) Unwrap() error { return vault.ErrLocked }

func (e *GPGFailure) UserMessage() string { return e.Reason.detail().notice }

func (e *GPGFailure) Error() string {
	if e.ExitCode >= 0 {
		return fmt.Sprintf("GPG %s (exit %d)", e.Reason.detail().description, e.ExitCode)
	}
	return "GPG " + e.Reason.detail().description
}

type gpgReasonDetail struct {
	description, notice string
	cause               debug.Cause
}

var gpgReasons = [...]gpgReasonDetail{
	gpgUnknown:          {"failed (cause not identified)", "GPG: unknown failure; run pass show", debug.Unknown},
	gpgNoSecretKey:      {"has no matching secret key", "GPG: missing secret key", debug.MissingKey},
	gpgNoPinentry:       {"cannot start pinentry", "GPG: pinentry unavailable", debug.Pinentry},
	gpgNoTerminal:       {"cannot access a terminal", "GPG: no terminal for pinentry", debug.Terminal},
	gpgAgentUnavailable: {"cannot contact gpg-agent", "GPG: agent unavailable", debug.Agent},
	gpgBadPassphrase:    {"rejected the passphrase", "GPG: passphrase rejected", debug.Passphrase},
	gpgCancelled:        {"prompt was cancelled", "GPG: prompt cancelled", debug.Canceled},
	gpgDecryptionFailed: {"could not decrypt this entry", "GPG: decryption failed", debug.Decrypt},
}

func (r GPGReason) detail() gpgReasonDetail {
	if int(r) >= len(gpgReasons) {
		return gpgReasons[gpgUnknown]
	}
	return gpgReasons[r]
}

func gpgReason(message string) GPGReason {
	switch {
	case strings.Contains(message, "no secret key"), strings.Contains(message, "secret key not found"):
		return gpgNoSecretKey
	case strings.Contains(message, "no pinentry"), strings.Contains(message, "pinentry not found"), strings.Contains(message, "cannot connect to pinentry"):
		return gpgNoPinentry
	case strings.Contains(message, "inappropriate ioctl"), strings.Contains(message, "no tty"), strings.Contains(message, "not a tty"):
		return gpgNoTerminal
	case strings.Contains(message, "gpg-agent"), strings.Contains(message, "no agent running"), strings.Contains(message, "problem with the agent"):
		return gpgAgentUnavailable
	case strings.Contains(message, "bad passphrase"), strings.Contains(message, "incorrect passphrase"):
		return gpgBadPassphrase
	case strings.Contains(message, "canceled"), strings.Contains(message, "cancelled"):
		return gpgCancelled
	case strings.Contains(message, "decryption failed"):
		return gpgDecryptionFailed
	default:
		return gpgUnknown
	}
}
