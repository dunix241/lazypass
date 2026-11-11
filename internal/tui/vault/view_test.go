package vault

import (
	"context"
	"errors"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type memoryVault struct {
	nodes map[string][]vault.Node
	read  map[string]vault.Entry
	store vault.Entry
	err   error
}

func (m *memoryVault) Name() string                     { return "memory" }
func (m *memoryVault) Capabilities() vault.Capabilities { return vault.Capabilities{} }
func (m *memoryVault) List(_ context.Context, p vault.Path) ([]vault.Node, error) {
	return m.nodes[p.String()], m.err
}
func (m *memoryVault) Read(_ context.Context, p vault.Path) (vault.Entry, error) {
	return m.read[p.String()], m.err
}
func (m *memoryVault) Write(_ context.Context, e vault.Entry) error { m.store = e; return m.err }

func (m *memoryVault) Create(ctx context.Context, entry vault.Entry) error {
	if err := (service.Service{Provider: m}).CheckNewPath(ctx, entry.Path); err != nil {
		return err
	}
	return m.Write(ctx, entry)
}
func (m *memoryVault) Delete(context.Context, vault.Path) error       { return nil }
func (m *memoryVault) Sync(context.Context) (vault.SyncResult, error) { return vault.SyncResult{}, nil }

func key(v *View, k tcell.Key, r rune) { v.HandleKey(tcell.NewEventKey(k, r, tcell.ModNone)) }
func testFrame() shared.Frame {
	return shared.Frame{Viewport: shared.Rect{W: 50, H: 18}, Card: shared.Rect{X: 1, Y: 1, W: 48, H: 16}, Palette: shared.DefaultPalette(), NerdFont: true}
}

func TestVaultBrowseCopyAndParentSelection(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "alpha", Path: vault.Path{"alpha"}, Kind: vault.FolderNode}, {Name: "team", Path: vault.Path{"team"}, Kind: vault.FolderNode}}, "team": {{Name: "site", Path: vault.Path{"team", "site"}, Kind: vault.EntryNode}}}, read: map[string]vault.Entry{"team/site": {Password: "secret"}}}
	var copied, message string
	v := New(Dependencies{Service: service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}, Notify: func(_ shared.Level, m string) { message = m }})
	v.Refresh()
	key(v, tcell.KeyRune, 'j')
	key(v, tcell.KeyRune, 'c')
	if copied != "team" || len(v.vaultPath) != 0 {
		t.Fatal("c did not copy folder name without navigating")
	}
	copied = ""
	key(v, tcell.KeyRune, 'l')
	if v.vaultPath.String() != "team" {
		t.Fatal("l did not open folder")
	}
	key(v, tcell.KeyRune, 'c')
	if copied != "secret" {
		t.Fatal("c did not copy entry value")
	}
	key(v, tcell.KeyRune, 'l')
	if copied != "secret" || message != "Copied to clipboard" {
		t.Fatal("entry copy failed")
	}
	copied = ""
	key(v, tcell.KeyEnter, 0)
	if copied != "secret" {
		t.Fatal("Enter did not copy")
	}
	key(v, tcell.KeyRune, 'h')
	if len(v.vaultPath) != 0 || v.vaultSelected != 1 {
		t.Fatal("parent selection not restored")
	}
}

func TestVaultFilterAndCopyDoesNotBlockNavigation(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "github", Path: vault.Path{"github"}, Kind: vault.EntryNode}, {Name: "gitlab", Path: vault.Path{"gitlab"}, Kind: vault.EntryNode}, {Name: "personal", Path: vault.Path{"personal"}, Kind: vault.FolderNode}}}, read: map[string]vault.Entry{"github": {Password: "one"}, "gitlab": {Password: "two"}}}
	var copied string
	v := New(Dependencies{Service: service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}})
	v.Refresh()
	key(v, tcell.KeyEnter, 0)
	key(v, tcell.KeyDown, 0)
	key(v, tcell.KeyEnter, 0)
	if copied != "two" {
		t.Fatal("copy blocked navigation")
	}
	key(v, tcell.KeyRune, '/')
	key(v, tcell.KeyRune, 'g')
	if len(v.filteredVaultNodes()) != 2 {
		t.Fatal("filter mismatch")
	}
	key(v, tcell.KeyDown, 0)
	key(v, tcell.KeyEnter, 0)
	if copied != "two" {
		t.Fatal("filtered picker mismatch")
	}
	key(v, tcell.KeyCtrlW, 0)
	if v.vaultFilter != "" {
		t.Fatal("Ctrl-W failed")
	}
	key(v, tcell.KeyEscape, 0)
	if v.IsFiltering() {
		t.Fatal("Escape did not clear filter")
	}
}

func TestVaultMouseMatchesScrolledRows(t *testing.T) {
	nodes := make([]vault.Node, 20)
	for i := range nodes {
		name := fmt.Sprintf("entry-%02d", i)
		nodes[i] = vault.Node{Name: name, Path: vault.Path{name}, Kind: vault.EntryNode}
	}
	v := New(Dependencies{Service: service.Service{Provider: &memoryVault{nodes: map[string][]vault.Node{"": nodes}}}})
	v.Refresh()
	f := testFrame()
	f.Card = f.Card.Translate(7, 2)
	for _, filtered := range []bool{false, true} {
		v.vaultFiltering, v.vaultFilter, v.vaultSelected = filtered, "entry", 18
		y, _, start := v.vaultListWindow(f.Card)
		v.HandleMouse(tview.MouseLeftClick, tcell.NewEventMouse(f.Card.X+2, y, tcell.Button1, tcell.ModNone), f)
		if v.vaultSelected != start {
			t.Fatal("mouse row did not match visible scroll window")
		}
	}
}

func TestVaultMouseSelectsAndActivates(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "alpha", Path: vault.Path{"alpha"}, Kind: vault.FolderNode}, {Name: "team", Path: vault.Path{"team"}, Kind: vault.FolderNode}}, "team": {{Name: "site", Path: vault.Path{"team", "site"}, Kind: vault.EntryNode}}}, read: map[string]vault.Entry{"team/site": {Password: "secret"}}}
	var copied string
	v := New(Dependencies{Service: service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}})
	v.Refresh()
	f := testFrame()
	mouse := func(action tview.MouseAction, row int) {
		v.HandleMouse(action, tcell.NewEventMouse(f.Card.X+2, f.Card.Y+2+row, tcell.Button1, tcell.ModNone), f)
	}
	mouse(tview.MouseLeftClick, 1)
	if v.vaultSelected != 1 || len(v.vaultPath) != 0 {
		t.Fatal("first click should select only")
	}
	mouse(tview.MouseLeftClick, 1)
	if v.vaultPath.String() != "team" {
		t.Fatal("second click should open")
	}
	mouse(tview.MouseLeftDoubleClick, 1)
	if copied != "secret" {
		t.Fatal("double click did not copy")
	}
	mouse(tview.MouseScrollDown, 0)
	mouse(tview.MouseScrollUp, 0)
	mouse(tview.MouseLeftClick, 0)
	mouse(tview.MouseLeftClick, 0)
	if len(v.vaultPath) != 0 {
		t.Fatal("parent row did not navigate")
	}
	mouse(tview.MouseLeftClick, 1)
	mouse(tview.MouseLeftClick, 1)
	mouse(tview.MouseRightClick, 0)
	if len(v.vaultPath) != 0 {
		t.Fatal("right click did not go back")
	}
}

func TestVaultNodeLabelsAndColors(t *testing.T) {
	folder := vault.Node{Name: "personal", Kind: vault.FolderNode}
	entry := vault.Node{Name: "proton", Kind: vault.EntryNode}
	colors := shared.DefaultPalette()
	if nodeLabel(folder, true) != " personal/" || nodeLabel(entry, true) != "󰦝 proton" || nodeLabel(folder, false) != "personal/" || nodeLabel(entry, false) != "proton" {
		t.Fatal("node labels changed")
	}
	if nodeColor(folder, colors) != colors.Accent || nodeColor(entry, colors) != colors.Text {
		t.Fatal("node colors changed")
	}
}

type safeDiagnosticError struct{}

func (safeDiagnosticError) Error() string       { return "secret-value in external error" }
func (safeDiagnosticError) Unwrap() error       { return vault.ErrLocked }
func (safeDiagnosticError) UserMessage() string { return "GPG: no terminal for pinentry" }

func TestCopyErrorMessageMapsFailureCauses(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{{vault.ErrLocked, "Vault locked (no details)"}, {safeDiagnosticError{}, "GPG: no terminal for pinentry"}, {vault.ErrNotFound, "Vault entry not found"}, {vault.ErrMalformed, "Vault entry format invalid"}, {vault.ErrUninitialized, "Vault not initialized"}, {vault.ErrUnavailable, "Vault unavailable"}, {service.ErrClipboard, "Clipboard unavailable"}, {errors.New("write failed"), "Copy failed"}} {
		if got := copyErrorMessage(test.err); got != test.want {
			t.Fatalf("message=%q want=%q", got, test.want)
		}
	}
	if storeErrorMessage(safeDiagnosticError{}) != "GPG: no terminal for pinentry" {
		t.Fatal("safe diagnostic lost")
	}
}

func TestVaultCopyFailureLogsOnlyClassifiedCause(t *testing.T) {
	state := t.TempDir()
	t.Setenv("LAZYPASS_DEBUG", "1")
	t.Setenv("XDG_STATE_HOME", state)
	p := &memoryVault{nodes: map[string][]vault.Node{"": {{Name: "mail", Path: vault.Path{"mail"}, Kind: vault.EntryNode}}}, read: map[string]vault.Entry{"mail": {Password: "secret-value"}}}
	var message string
	v := New(Dependencies{Service: service.Service{Provider: p, Copy: func(string) error { return errors.New("secret-value in failure") }}, Notify: func(_ shared.Level, m string) { message = m }})
	v.Refresh()
	key(v, tcell.KeyEnter, 0)
	if message != "Clipboard unavailable" {
		t.Fatal("unsafe copy notification")
	}
	log, err := os.ReadFile(filepath.Join(state, "lazypass", "debug.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), `cause="clipboard"`) || strings.Contains(string(log), "secret-value") {
		t.Fatal("unsafe debug log")
	}
}
