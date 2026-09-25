package theme

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBuiltinsAreValidAndLoadable(t *testing.T) {
	for _, name := range []string{"midnight-rose", "nord", "mono", "gruvbox", "catppuccin-mocha", "onedark", "dracula", "tokyo-night", "rose-pine"} {
		palette, err := Load(name)
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		if err := palette.Validate(); err != nil {
			t.Fatalf("validate %s: %v", name, err)
		}
	}
}

func TestValidateName(t *testing.T) {
	for _, name := range []string{"a", "my-theme", "v2"} {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q): %v", name, err)
		}
	}
	for _, name := range []string{"", "UPPER", "two words", "../escape", "under_score", "name.yaml"} {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) succeeded", name)
		}
	}
}

func TestValidatePaletteRequiresStrictHexColors(t *testing.T) {
	palette, err := Load(DefaultName())
	if err != nil {
		t.Fatal(err)
	}
	palette.Accent = "#abc"
	if err := palette.Validate(); err == nil {
		t.Fatal("short hex color was accepted")
	}
	palette.Accent = "#ABCDEF"
	if err := palette.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCustomThemeLifecycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	palette, err := Load("nord")
	if err != nil {
		t.Fatal(err)
	}
	if err := Save("my-theme", palette); err != nil {
		t.Fatalf("save: %v", err)
	}
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "my-theme.yaml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("theme permissions = %o, want 600", info.Mode().Perm())
	}
	loaded, err := Load("my-theme")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(loaded, palette) {
		t.Fatalf("loaded = %+v, want %+v", loaded, palette)
	}
	names, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"catppuccin-mocha", "dracula", "gruvbox", "midnight-rose", "mono", "my-theme", "nord", "onedark", "rose-pine", "tokyo-night"}) {
		t.Fatalf("names = %v", names)
	}
	if err := Delete("my-theme"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := Load("my-theme"); !os.IsNotExist(err) {
		t.Fatalf("load deleted theme error = %v", err)
	}
}

func TestCustomThemesRejectMalformedAndUnsafeFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("base: '#000000'\nextra: '#FFFFFF'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("bad"); err == nil {
		t.Fatal("malformed theme was accepted")
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "linked.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("linked"); err == nil {
		t.Fatal("symlink theme was accepted")
	}
	if err := Delete("linked"); err == nil {
		t.Fatal("symlink theme was deleted")
	}
}
