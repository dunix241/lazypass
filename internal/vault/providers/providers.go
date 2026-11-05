// Package providers is the composition boundary for built-in vault backends.
package providers

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"lazypass/internal/config"
	"lazypass/internal/vault"
	ageprovider "lazypass/internal/vault/age"
	"lazypass/internal/vault/bitwarden"
	passprovider "lazypass/internal/vault/pass"
)

// Managed adds setup inspection to the transport-neutral provider contract.
type Managed interface {
	vault.Provider
	Status(context.Context) error
	StoreDirectory() string
}

type Prompt func() (string, error)
type Factory func(config.Vault, Prompt) (Managed, error)

type Registry map[string]Factory

func Builtins() Registry {
	return Registry{
		"bitwarden": func(entry config.Vault, prompt Prompt) (Managed, error) {
			if entry.Server != "" {
				u, err := url.Parse(entry.Server)
				if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
					return nil, fmt.Errorf("bitwarden server must be an HTTPS URL without credentials, query, or fragment")
				}
			}
			dir := entry.StoreDir
			if dir == "" {
				base, err := os.UserConfigDir()
				if err != nil {
					return nil, err
				}
				dir = filepath.Join(base, "lazypass", "bitwarden", entry.Name)
			}
			return bitwarden.New(dir, entry.Server, bitwarden.Prompt(prompt), nil), nil
		},
		"pass": func(entry config.Vault, _ Prompt) (Managed, error) {
			return passprovider.New(entry.StoreDir, nil), nil
		},
		"age": func(entry config.Vault, prompt Prompt) (Managed, error) {
			store, identity, err := AgePaths(entry)
			if err != nil {
				return nil, err
			}
			return ageprovider.New(store, identity, ageprovider.Prompt(prompt)), nil
		},
	}
}

func (r Registry) Open(entry config.Vault, prompt Prompt) (Managed, error) {
	if entry.Name != "" {
		if err := config.ValidateVaultName(entry.Name); err != nil {
			return nil, err
		}
	}
	factory, ok := r[entry.Provider]
	if !ok {
		return nil, fmt.Errorf("unsupported vault provider %q", entry.Provider)
	}
	return factory(entry, prompt)
}

func AgePaths(entry config.Vault) (store, identity string, err error) {
	store, identity = entry.StoreDir, entry.IdentityFile
	suffix := ""
	if entry.Name != "" && entry.Name != "age" {
		suffix = "-" + entry.Name
	}
	if store == "" {
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, e := os.UserHomeDir()
			if e != nil {
				return "", "", e
			}
			base = filepath.Join(home, ".local", "share")
		}
		store = filepath.Join(base, "lazypass", "vault"+suffix)
	}
	if identity == "" {
		base, e := os.UserConfigDir()
		if e != nil {
			return "", "", e
		}
		identity = filepath.Join(base, "lazypass", "identity"+suffix+".age")
	}
	return store, identity, nil
}
