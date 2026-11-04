// Package vault defines the provider-neutral password vault contract.
package vault

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var (
	ErrNotFound       = errors.New("vault entry not found")
	ErrLocked         = errors.New("vault is locked")
	ErrConflict       = errors.New("vault sync conflict")
	ErrUnavailable    = errors.New("vault is unavailable")
	ErrUninitialized  = errors.New("vault is not initialized")
	ErrInvalidPath    = errors.New("invalid vault path")
	ErrMalformed      = errors.New("malformed vault entry")
	ErrUnsupported    = errors.New("vault operation unsupported")
	ErrPromptRequired = errors.New("vault requires an interactive prompt")
)

type noninteractiveKey struct{}

type passphrasePromptKey struct{}
type PassphrasePrompt func(context.Context) (string, error)

func WithPassphrasePrompt(ctx context.Context, prompt PassphrasePrompt) context.Context {
	return context.WithValue(ctx, passphrasePromptKey{}, prompt)
}

func RequestPassphrase(ctx context.Context, fallback func() (string, error)) (string, error) {
	if prompt, ok := ctx.Value(passphrasePromptKey{}).(PassphrasePrompt); ok {
		return prompt(ctx)
	}
	if fallback == nil {
		return "", ErrLocked
	}
	return fallback()
}

type PassphraseProvider interface{ UsesPassphrasePrompt() }

// WithoutPrompt lets transports try cached credentials without taking the TTY.
func WithoutPrompt(ctx context.Context) context.Context {
	return context.WithValue(ctx, noninteractiveKey{}, true)
}

func IsNoninteractive(ctx context.Context) bool {
	value, _ := ctx.Value(noninteractiveKey{}).(bool)
	return value
}

type Path []string

func ParsePath(text string) (Path, error) {
	if text == "" {
		return nil, nil
	}
	path := Path(strings.Split(text, "/"))
	if err := path.Validate(); err != nil {
		return nil, err
	}
	return path, nil
}

func (p Path) Validate() error {
	for _, part := range p {
		if part == ".lazypass" {
			return ErrInvalidPath
		}
		if part == "" || part == "." || part == ".." || part == ".gpg-id" || part == ".git" || strings.ContainsAny(part, `/\\`) || strings.HasSuffix(part, ".gpg") {
			return fmt.Errorf("%w: invalid component", ErrInvalidPath)
		}
		for _, r := range part {
			if unicode.IsControl(r) {
				return fmt.Errorf("%w: control character", ErrInvalidPath)
			}
		}
	}
	return nil
}

func (p Path) String() string { return strings.Join(p, "/") }

type NodeKind uint8

const (
	FolderNode NodeKind = iota
	EntryNode
)

type Node struct {
	EntryType EntryType
	TypeKnown bool
	Path      Path
	Name      string
	Kind      NodeKind
}

type Entry struct {
	Preserved *EntryText `json:"-"`
	Type      EntryType
	Path      Path
	Password  string
	Username  string
	URL       string
	Notes     string
	Fields    map[string]string
}

// EntryText retains externally authored text during password-only updates.
type EntryText struct {
	Tail     string
	Username string
	URL      string
	Notes    string
	Fields   map[string]string
}

type EntryType uint8

func AllEntryTypes() []EntryType {
	return []EntryType{LoginEntry, CardEntry, IdentityEntry, NoteEntry, SSHKeyEntry}
}

const (
	LoginEntry EntryType = iota
	CardEntry
	IdentityEntry
	NoteEntry
	SSHKeyEntry
)

func (t EntryType) String() string {
	switch t {
	case CardEntry:
		return "Card"
	case IdentityEntry:
		return "Identity"
	case NoteEntry:
		return "Note"
	case SSHKeyEntry:
		return "SSH key"
	default:
		return "Login"
	}
}

type Capabilities struct {
	Sync, Delete bool
}

type EntryTypes interface{ SupportedEntryTypes() []EntryType }

type SyncResult struct {
	Pulled bool
	Pushed bool
}

// Diagnostic errors provide safe, provider-specific explanations for users.
type Diagnostic interface {
	error
	UserMessage() string
}

type Provider interface {
	Name() string
	Capabilities() Capabilities
	List(context.Context, Path) ([]Node, error)
	Read(context.Context, Path) (Entry, error)
	Write(context.Context, Entry) error
	Delete(context.Context, Path) error
	Sync(context.Context) (SyncResult, error)
}

// FolderCreator is optional for backends which support empty folders.
type FolderCreator interface {
	CreateFolder(context.Context, Path) error
}

// EntryCreator publishes a new entry without replacing an existing entry.
type EntryCreator interface {
	Create(context.Context, Entry) error
}

// Locker drops any session-held decryption identities.
type Locker interface{ Lock() }

type FolderDeleter interface {
	DeleteFolder(context.Context, Path) error
}

type MetadataWriter interface {
	WriteMetadata(context.Context, Path, EntryType) error
}

type MetadataReader interface {
	DecorateMetadata([]Node)
}

type SavedWarning struct{}

func (SavedWarning) Error() string {
	return "Entry saved, but metadata or Git bookkeeping needs repair"
}
func (e SavedWarning) UserMessage() string { return e.Error() }
