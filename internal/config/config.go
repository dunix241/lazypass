// Package config persists last-used options to XDG config dir.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lazypass/internal/debug"
	"lazypass/internal/generator"
	"lazypass/internal/theme"
	"lazypass/internal/util"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

// Config is the persisted form of generator.Options.
type Config struct {
	Updates          Updates `yaml:"updates"`
	Version          int     `yaml:"version"`
	Length           int     `yaml:"length"`
	Upper            bool    `yaml:"upper"`
	Lower            bool    `yaml:"lower"`
	Numbers          bool    `yaml:"numbers"`
	Symbols          bool    `yaml:"symbols"`
	SymbolSet        string  `yaml:"symbolSet"`
	ExcludeAmbiguous bool    `yaml:"excludeAmbiguous"`
	Theme            string  `yaml:"theme"`
	NerdFont         bool    `yaml:"nerdFont"`
	Vaults           Vaults  `yaml:"vaults"`
	LastVault        string  `yaml:"-"` // Legacy read-only migration value.
}

type Updates struct {
	Check bool `yaml:"check"`
}

func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	type plain Config
	decoded := plain(*c)
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	*c = Config(decoded)
	var legacyVault *yaml.Node
	hasVaults := false
	for i := 0; i+1 < len(node.Content); i += 2 {
		switch node.Content[i].Value {
		case "lastVault":
			if err := node.Content[i+1].Decode(&c.LastVault); err != nil {
				return err
			}
		case "vaults":
			hasVaults = true
		case "vault":
			legacyVault = node.Content[i+1]
		}
	}
	if !hasVaults && legacyVault != nil {
		return legacyVault.Decode(&c.Vaults)
	}
	return nil
}

// Vault selects an optional password vault backend.
type Vault struct {
	Name         string `yaml:"name"`
	Provider     string `yaml:"provider"`
	StoreDir     string `yaml:"storeDir"`
	IdentityFile string `yaml:"identityFile,omitempty"`
	Server       string `yaml:"server,omitempty"`
}

type Vaults []Vault

func ValidateVaultName(name string) error {
	if name == "" || strings.Trim(name, "abcdefghijklmnopqrstuvwxyz0123456789-") != "" {
		return fmt.Errorf("vault names must use lowercase letters, numbers, and hyphens")
	}
	return nil
}

func (v *Vaults) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.MappingNode {
		var legacy Vault
		if err := node.Decode(&legacy); err != nil {
			return err
		}
		if legacy.Provider != "" {
			if legacy.Name == "" {
				legacy.Name = legacy.Provider
			}
			*v = Vaults{legacy}
		}
		return nil
	}
	type plain Vaults
	var decoded plain
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	*v = Vaults(decoded)
	if len(*v) == 0 {
		*v = nil
	}
	return nil
}

func (v Vaults) Select(name string) (Vault, bool) {
	for _, entry := range v {
		if name == "" || entry.Name == name {
			return entry, true
		}
	}
	return Vault{}, false
}

func (c Config) SelectedVault(name string) (Vault, bool) {
	if name != "" {
		return c.Vaults.Select(name)
	}
	if c.LastVault != "" {
		if entry, ok := c.Vaults.Select(c.LastVault); ok {
			return entry, true
		}
	}
	return c.Vaults.Select("")
}

// Defaults for the first run: length 20, upper+lower+numbers.
func Defaults() Config {
	d := generator.Defaults()
	return Config{
		Updates:          Updates{Check: true},
		Version:          1,
		Length:           d.Length,
		Upper:            d.Upper,
		Lower:            d.Lower,
		Numbers:          d.Numbers,
		Symbols:          d.Symbols,
		SymbolSet:        d.SymbolSet,
		ExcludeAmbiguous: d.ExcludeAmbiguous,
		Theme:            theme.DefaultName(),
		NerdFont:         true,
	}
}

func (c Config) ToOptions() generator.Options {
	return generator.Options{
		Length:           c.Length,
		Upper:            c.Upper,
		Lower:            c.Lower,
		Numbers:          c.Numbers,
		Symbols:          c.Symbols,
		SymbolSet:        c.SymbolSet,
		ExcludeAmbiguous: c.ExcludeAmbiguous,
	}
}

// WithOptions replaces password generator settings while preserving other config.
func (c Config) WithOptions(o generator.Options) Config {
	c.Length = o.Length
	c.Upper = o.Upper
	c.Lower = o.Lower
	c.Numbers = o.Numbers
	c.Symbols = o.Symbols
	c.SymbolSet = o.SymbolSet
	c.ExcludeAmbiguous = o.ExcludeAmbiguous
	return c
}

func DefaultPath() (string, error) {
	dir := filepath.Join(xdg.ConfigHome, "lazypass")
	return filepath.Join(dir, "config.yaml"), nil
}

// ResolvePath precedence: explicit flag > LAZYPASS_CONFIG > XDG default.
func ResolvePath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if env := os.Getenv("LAZYPASS_CONFIG"); env != "" {
		return env, nil
	}
	return DefaultPath()
}

// Load returns defaults for a missing file. Invalid existing files are left
// untouched and reported to the caller so user settings are never replaced.
func Load(path string) (Config, error) {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return Defaults(), err
		}
		path = p
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Defaults(), nil
		}
		debug.Failure("config load", debug.IO)
		return Defaults(), err
	}
	c := Defaults()
	if err := yaml.Unmarshal(data, &c); err != nil {
		debug.Failure("config parse", debug.Invalid)
		return Defaults(), fmt.Errorf("invalid config %q: %w", path, err)
	}
	if c.SymbolSet == "" {
		c.SymbolSet = generator.DefaultSymbols
	}
	if c.Theme == "" {
		c.Theme = theme.DefaultName()
	}
	seen := map[string]bool{}
	for i := range c.Vaults {
		entry := &c.Vaults[i]
		if entry.Name == "" {
			entry.Name = entry.Provider
		}
		if ValidateVaultName(entry.Name) != nil || seen[entry.Name] {
			return Defaults(), fmt.Errorf("invalid config: vault names must be nonempty and unique")
		}
		seen[entry.Name] = true
	}
	if err := c.ToOptions().Validate(); err != nil {
		debug.Failure("config validate", debug.Invalid)
		return Defaults(), fmt.Errorf("invalid config %q: %w", path, err)
	}
	return c, nil
}

// Save writes atomically (tmp + rename) with 0600 permissions.
func (c Config) Save(path string) error {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return err
		}
		path = p
	}
	var data bytes.Buffer
	encoder := yaml.NewEncoder(&data)
	encoder.SetIndent(2)
	if err := encoder.Encode(c); err != nil {
		debug.Failure("config encode", debug.Invalid)
		return err
	}
	if err := encoder.Close(); err != nil {
		debug.Failure("config encode", debug.IO)
		return err
	}
	if err := util.WritePrivateFile(path, data.Bytes()); err != nil {
		debug.Failure("config save file", debug.IO)
		return err
	}
	return nil
}
