// Package theme manages lazypass's semantic color palettes.
package theme

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultName = "midnight-rose"

const maxThemeSize = 1 << 20

var namePattern = regexp.MustCompile(`^[a-z0-9-]+$`)
var colorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Palette contains the colors used for each UI semantic role.
type Palette struct {
	Base    string `yaml:"base"`
	Surface string `yaml:"surface"`
	Border  string `yaml:"border"`
	Text    string `yaml:"text"`
	Muted   string `yaml:"muted"`
	Accent  string `yaml:"accent"`
	Focus   string `yaml:"focus"`
	Warning string `yaml:"warning"`
}

var builtins = map[string]Palette{
	"midnight-rose": {
		Base: "#15111F", Surface: "#211A2F", Border: "#52416D", Text: "#F4EFFA",
		Muted: "#B5A8CB", Accent: "#FF8FAB", Focus: "#CA9DF6", Warning: "#FBBF7E",
	},
	"nord": {
		Base: "#2E3440", Surface: "#3B4252", Border: "#4C566A", Text: "#ECEFF4",
		Muted: "#D8DEE9", Accent: "#88C0D0", Focus: "#81A1C1", Warning: "#EBCB8B",
	},
	"mono": {
		Base: "#000000", Surface: "#1A1A1A", Border: "#555555", Text: "#FFFFFF",
		Muted: "#AAAAAA", Accent: "#FFFFFF", Focus: "#FFFFFF", Warning: "#FFFFFF",
	},
	"gruvbox": {
		Base: "#282828", Surface: "#3C3836", Border: "#665C54", Text: "#EBDBB2",
		Muted: "#A89984", Accent: "#FE8019", Focus: "#83A598", Warning: "#FABD2F",
	},
	"catppuccin-mocha": {
		Base: "#1E1E2E", Surface: "#313244", Border: "#585B70", Text: "#CDD6F4",
		Muted: "#A6ADC8", Accent: "#F38BA8", Focus: "#CBA6F7", Warning: "#F9E2AF",
	},
	"onedark": {
		Base: "#282C34", Surface: "#21252B", Border: "#3E4451", Text: "#ABB2BF",
		Muted: "#5C6370", Accent: "#61AFEF", Focus: "#C678DD", Warning: "#E5C07B",
	},
	"dracula": {
		Base: "#282A36", Surface: "#44475A", Border: "#6272A4", Text: "#F8F8F2",
		Muted: "#BFBFBF", Accent: "#FF79C6", Focus: "#BD93F9", Warning: "#F1FA8C",
	},
	"tokyo-night": {
		Base: "#1A1B26", Surface: "#24283B", Border: "#414868", Text: "#C0CAF5",
		Muted: "#565F89", Accent: "#7AA2F7", Focus: "#BB9AF7", Warning: "#E0AF68",
	},
	"rose-pine": {
		Base: "#191724", Surface: "#1F1D2E", Border: "#403D52", Text: "#E0DEF4",
		Muted: "#908CAA", Accent: "#EBBCBA", Focus: "#C4A7E7", Warning: "#F6C177",
	},
}

// DefaultName is the palette selected by a new configuration.
func DefaultName() string { return defaultName }

// IsBuiltin reports whether name identifies an embedded palette.
func IsBuiltin(name string) bool {
	_, ok := builtins[name]
	return ok
}

// ValidateName accepts lowercase letters, numbers, and hyphens only.
func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid theme name %q: use lowercase letters, numbers, and hyphens", name)
	}
	return nil
}

// Validate ensures every role has a strict #RRGGBB color.
func (p Palette) Validate() error {
	for _, role := range []struct {
		name  string
		color string
	}{
		{"base", p.Base}, {"surface", p.Surface}, {"border", p.Border}, {"text", p.Text},
		{"muted", p.Muted}, {"accent", p.Accent}, {"focus", p.Focus}, {"warning", p.Warning},
	} {
		if !colorPattern.MatchString(role.color) {
			return fmt.Errorf("invalid %s color %q: use #RRGGBB", role.name, role.color)
		}
	}
	return nil
}

// Dir returns the XDG directory used for custom themes.
func Dir() (string, error) {
	home := os.Getenv("XDG_CONFIG_HOME")
	if home == "" {
		var err error
		home, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(home, "lazypass", "themes"), nil
}

// List returns built-in and valid custom theme names in lexical order.
func List() ([]string, error) {
	names := make(map[string]struct{}, len(builtins))
	for name := range builtins {
		names[name] = struct{}{}
	}
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sortedNames(names), nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && filepath.Ext(entry.Name()) == ".yaml" {
			name := strings.TrimSuffix(entry.Name(), ".yaml")
			if ValidateName(name) == nil {
				names[name] = struct{}{}
			}
		}
	}
	return sortedNames(names), nil
}

func sortedNames(names map[string]struct{}) []string {
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

// Load resolves a built-in palette or reads a valid custom YAML palette.
func Load(name string) (Palette, error) {
	if err := ValidateName(name); err != nil {
		return Palette{}, err
	}
	if palette, ok := builtins[name]; ok {
		return palette, nil
	}
	path, err := pathFor(name)
	if err != nil {
		return Palette{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return Palette{}, err
	}
	if !info.Mode().IsRegular() {
		return Palette{}, fmt.Errorf("theme %q is not a regular file", name)
	}
	if info.Size() > maxThemeSize {
		return Palette{}, fmt.Errorf("theme %q exceeds %d bytes", name, maxThemeSize)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Palette{}, err
	}
	var palette Palette
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&palette); err != nil {
		return Palette{}, fmt.Errorf("decoding theme %q: %w", name, err)
	}
	if err := palette.Validate(); err != nil {
		return Palette{}, fmt.Errorf("validating theme %q: %w", name, err)
	}
	return palette, nil
}

// Save atomically writes a custom palette with private file permissions.
func Save(name string, palette Palette) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if _, ok := builtins[name]; ok {
		return fmt.Errorf("%q is a built-in theme", name)
	}
	if err := palette.Validate(); err != nil {
		return err
	}
	path, err := pathFor(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(palette)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+name+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// Delete removes a custom theme without following symlinks.
func Delete(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if _, ok := builtins[name]; ok {
		return fmt.Errorf("%q is a built-in theme", name)
	}
	path, err := pathFor(name)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("theme %q is not a regular file", name)
	}
	return os.Remove(path)
}

func pathFor(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".yaml"), nil
}
