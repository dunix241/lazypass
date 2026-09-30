package tui

import (
	"errors"
	"time"
	"unicode/utf8"

	"lazypass/internal/debug"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
)

type notificationLevel uint8

const (
	notificationInfo notificationLevel = iota
	notificationSuccess
	notificationWarning
	notificationError
)

const (
	generatorFooterFull    = "<tab> focus  •  a add  •  r regenerate  •  c copy  •  t themes  •  v view  •  q quit"
	generatorFooterCompact = "a add  •  c copy  •  t themes  •  v view  •  q quit"
	generatorFooterShort   = "a add  •  v view  •  q quit"
	generatorFooterMinimal = "q quit"
	vaultFooter            = "h back  •  l open/copy  •  / filter  •  t themes  •  v view  •  q quit"
	vaultCompact           = "h back  •  l open/copy  •  / filter  •  v view  •  q quit"
	vaultShort             = "h back  •  l open/copy  •  v view  •  q quit"
	vaultMinimal           = "h back • l open/copy • q quit"
)

func (s *screen) notify(level notificationLevel, message string) {
	s.status, s.statusLevel, s.statusTill = message, level, time.Now().Add(2*time.Second)
}

func (s *screen) generatorFooter(width int) string {
	contextHint := ""
	switch s.selected {
	case focusLength:
		contextHint = "<left>/<right> length"
	case focusUpper, focusLower, focusNumbers, focusSymbols, focusAmbiguous:
		contextHint = "<space> toggle"
	}
	bases := []string{generatorFooterFull, generatorFooterCompact, generatorFooterShort, generatorFooterMinimal}
	if contextHint == "" {
		return fitFooter(width, bases...)
	}
	candidates := make([]string, 0, len(bases)+1)
	for _, base := range bases {
		candidates = append(candidates, contextHint+"  •  "+base)
	}
	candidates = append(candidates, contextHint)
	return fitFooter(width, candidates...)
}

func fitFooter(width int, options ...string) string {
	for _, option := range options {
		if utf8.RuneCountInString(option) <= width-2 {
			return option
		}
	}
	return options[len(options)-1]
}

func (s *screen) drawHeaderStatus(screen tcell.Screen, logo logoLayout, x, y, width int) {
	if s.status != "" && time.Now().Before(s.statusTill) {
		statusX := x + width - utf8.RuneCountInString(s.status) - 2
		if statusX > x+logo.right()+2 {
			printAt(screen, statusX, y, s.status, s.notificationColor(s.statusLevel))
		} else {
			rightWidth := width - logo.right() - 2
			leftWidth := logo.x - 2
			if rightWidth >= leftWidth {
				printAt(screen, x+logo.right()+1, y, truncate(s.status, rightWidth), s.notificationColor(s.statusLevel))
			} else {
				printAt(screen, x+1, y, truncate(s.status, leftWidth), s.notificationColor(s.statusLevel))
			}
		}
	}
}

func (s *screen) notificationColor(level notificationLevel) tcell.Color {
	switch level {
	case notificationSuccess:
		return s.colors.accent
	case notificationWarning:
		return s.colors.warning
	case notificationError:
		return tcell.ColorRed
	default:
		return s.colors.focus
	}
}

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
