package bitwarden

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"lazypass/internal/vault"
)

func (p *Provider) signedOut() error {
	command := "BITWARDENCLI_APPDATA_DIR=" + "'" + strings.ReplaceAll(p.DataDir, "'", "'\"'\"'") + "' bw login"
	return &loginRequired{commandFailure{vault.ErrUninitialized, "Bitwarden is signed out. Run " + command + ", then refresh the vault"}, command}
}

type loginRequired struct {
	commandFailure
	command string
}

func (e *loginRequired) RecoveryCommand() string { return e.command }

type commandFailure struct {
	cause   error
	message string
}

func (e *commandFailure) Error() string       { return e.message }
func (e *commandFailure) UserMessage() string { return e.message }
func (e *commandFailure) Unwrap() error       { return e.cause }

func classifyFailure(ctx context.Context, output []byte, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &commandFailure{context.DeadlineExceeded, "Bitwarden request timed out; retry when the connection is available"}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, exec.ErrNotFound) {
		return &commandFailure{vault.ErrUnavailable, "Bitwarden CLI is missing; install bw"}
	}
	var exit *exec.ExitError
	message := string(output)
	if errors.As(err, &exit) {
		message += " " + string(exit.Stderr)
	}
	message = strings.ToLower(message)
	switch {
	case strings.Contains(message, "statuscode: 403"), strings.Contains(message, "status code: 403"), strings.Contains(message, "http 403"):
		return accessDenied()
	case strings.Contains(message, "forbidden"), strings.Contains(message, "permission denied"), strings.Contains(message, "not authorized"), strings.Contains(message, "do not have permission"):
		return &commandFailure{vault.ErrUnsupported, "Your account does not have permission for this Bitwarden operation"}
	case strings.Contains(message, "not logged in"), strings.Contains(message, "unauthenticated"), strings.Contains(message, "login required"):
		return &commandFailure{vault.ErrUninitialized, "Bitwarden is signed out; log in with bw using this vault's CLI directory"}
	case strings.Contains(message, "invalid master password"), strings.Contains(message, "invalid password"):
		return &commandFailure{vault.ErrLocked, "Bitwarden master password was rejected"}
	case strings.Contains(message, "invalid session"), strings.Contains(message, "vault is locked"), strings.Contains(message, "session key is invalid"):
		return &commandFailure{vault.ErrLocked, "Bitwarden session expired; unlock and retry"}
	case strings.Contains(message, "econnrefused"), strings.Contains(message, "enotfound"), strings.Contains(message, "fetch failed"), strings.Contains(message, "network"), strings.Contains(message, "timed out"):
		return &commandFailure{vault.ErrUnavailable, "Cannot reach Bitwarden; check your connection and server"}
	case strings.Contains(message, "validation"), strings.Contains(message, "is required"), strings.Contains(message, "bad request"):
		return &commandFailure{vault.ErrMalformed, "Bitwarden rejected the entry; check its required fields"}
	default:
		return &commandFailure{vault.ErrUnavailable, "Bitwarden operation failed; check your connection and account"}
	}
}
