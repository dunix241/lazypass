// Package config persists last-used options to XDG config dir.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"lazypass/internal/generator"
	"lazypass/internal/theme"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

// Config is the persisted form of generator.Options.
type Config struct {
	Version          int    `yaml:"version"`
	Length           int    `yaml:"length"`
	Upper            bool   `yaml:"upper"`
	Lower            bool   `yaml:"lower"`
	Numbers          bool   `yaml:"numbers"`
	Symbols          bool   `yaml:"symbols"`
	SymbolSet        string `yaml:"symbolSet"`
	ExcludeAmbiguous bool   `yaml:"excludeAmbiguous"`
	Theme            string `yaml:"theme"`
	NerdFont         bool   `yaml:"nerdFont"`
	Vault            Vault  `yaml:"vault"`
}

// Vault selects an optional password vault backend.
type Vault struct {
	Provider string `yaml:"provider"`
	StoreDir string `yaml:"storeDir"`
}

// Defaults for the first run: length 20, upper+lower+numbers.
func Defaults() Config {
	d := generator.Defaults()
	return Config{
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
		return Defaults(), err
	}
	c := Defaults()
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Defaults(), fmt.Errorf("invalid config %q: %w", path, err)
	}
	if c.SymbolSet == "" {
		c.SymbolSet = generator.DefaultSymbols
	}
	if c.Theme == "" {
		c.Theme = theme.DefaultName()
	}
	if err := c.ToOptions().Validate(); err != nil {
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
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var data bytes.Buffer
	encoder := yaml.NewEncoder(&data)
	encoder.SetIndent(2)
	if err := encoder.Encode(c); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data.Bytes(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
