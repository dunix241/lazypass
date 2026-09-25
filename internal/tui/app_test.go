package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

import "lazypass/internal/config"

type memoryVault struct {
	nodes map[string][]vault.Node
	read  map[string]vault.Entry
	store vault.Entry
	err   error
}

func (m *memoryVault) Name() string                     { return "memory" }
func (m *memoryVault) Capabilities() vault.Capabilities { return vault.Capabilities{} }
func (m *memoryVault) List(_ context.Context, path vault.Path) ([]vault.Node, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.nodes[path.String()], nil
}
func (m *memoryVault) Read(_ context.Context, path vault.Path) (vault.Entry, error) {
	return m.read[path.String()], m.err
}
func (m *memoryVault) Write(_ context.Context, entry vault.Entry) error {
	m.store = entry
	return m.err
}
func (m *memoryVault) Delete(context.Context, vault.Path) error { return nil }
func (m *memoryVault) Sync(context.Context) (vault.SyncResult, error) {
	return vault.SyncResult{}, nil
}

func TestRegenerateProducesPassword(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	if err := a.screen.regenerate(); err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if len(a.CurrentPassword()) != 20 {
		t.Fatalf("got len %d want 20", len(a.CurrentPassword()))
	}
}

func TestToggleKeepsOneClass(t *testing.T) {
	c := config.Defaults()
	c.Upper, c.Lower, c.Numbers, c.Symbols = true, false, false, false
	a := NewApp(c, "", nil)
	a.screen.selected = focusUpper
	before := a.CurrentPassword()
	a.screen.activate() // would disable the last class → must revert
	if !a.CurrentOptions().Upper {
		t.Fatal("should keep last class enabled")
	}
	if a.CurrentPassword() != before {
		t.Fatal("rejecting the toggle must not regenerate the password")
	}
}

func TestLengthTypingSupportsMoreThanFifty(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	a.screen.lengthText = ""
	a.screen.typeLength('1')
	a.screen.typeLength('2')
	a.screen.typeLength('8')
	if got := a.CurrentOptions().Length; got != 128 {
		t.Fatalf("got length %d, want 128", got)
	}
}

func TestInitialRenderDoesNotCreateConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewApp(config.Defaults(), path, nil)
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("initial render should not save config, got %v", err)
	}
}

func TestTUIOptionChangesPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	a := NewApp(config.Defaults(), path, nil)
	a.screen.setLength(128)
	a.screen.selected = focusSymbols
	a.screen.activate()
	a.screen.selected = focusAmbiguous
	a.screen.activate()

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Length != 128 || !loaded.Symbols || !loaded.ExcludeAmbiguous {
		t.Fatalf("saved options were not restored: %+v", loaded)
	}
}

func TestSavePreservesLatestNonGeneratorConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	current := config.Defaults()
	current.Vault = config.Vault{Provider: "pass", StoreDir: "/vault"}
	current.NerdFont = false
	if err := current.Save(path); err != nil {
		t.Fatal(err)
	}
	a := NewApp(config.Defaults(), path, nil)
	a.screen.cfg.Length = 32
	a.screen.save()
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Length != 32 || loaded.Vault != current.Vault || loaded.NerdFont != current.NerdFont {
		t.Fatalf("saved config = %#v", loaded)
	}
}

func TestCopyStatusReflectsClipboardOutcome(t *testing.T) {
	success := NewApp(config.Defaults(), "", func(string) error { return nil })
	success.screen.password = "password"
	success.screen.copy()
	if success.screen.status != "Copied to clipboard" || success.screen.statusLevel != notificationSuccess {
		t.Fatalf("success notification = %q/%d", success.screen.status, success.screen.statusLevel)
	}

	failure := NewApp(config.Defaults(), "", func(string) error { return errors.New("no clipboard") })
	failure.screen.password = "password"
	failure.screen.copy()
	if failure.screen.status != "Copy failed" || failure.screen.statusLevel != notificationError {
		t.Fatalf("failure notification = %q/%d", failure.screen.status, failure.screen.statusLevel)
	}
}

func TestNotificationLevelsUseDistinctColors(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	if a.screen.notificationColor(notificationInfo) != a.screen.colors.focus ||
		a.screen.notificationColor(notificationSuccess) != a.screen.colors.accent ||
		a.screen.notificationColor(notificationWarning) != a.screen.colors.warning ||
		a.screen.notificationColor(notificationError) != tcell.ColorRed {
		t.Fatal("notification levels should use their semantic colors")
	}
}

func TestGlobalOptionShortcuts(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	a.screen.selected = focusSymbols
	handler := a.screen.InputHandler()
	before := a.CurrentOptions().Length
	handler(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone), nil)
	if a.screen.selected != focusLength {
		t.Fatal("Right should adjust and focus Length")
	}
	if got := a.CurrentOptions().Length; got != before+1 {
		t.Fatalf("Right increased length to %d, want %d", got, before+1)
	}
	handler(tcell.NewEventKey(tcell.KeyRune, 'U', tcell.ModShift), nil)
	if a.CurrentOptions().Upper {
		t.Fatal("U should toggle Uppercase")
	}
	handler(tcell.NewEventKey(tcell.KeyRune, 'L', tcell.ModShift), nil)
	if a.CurrentOptions().Lower {
		t.Fatal("Shift+L should toggle Lowercase")
	}
	handler(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone), nil)
	if !a.CurrentOptions().Lower {
		t.Fatal("l should toggle Lowercase")
	}
}

func TestLengthFocusIsRenderedAfterGlobalShortcut(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	a.screen.selected = focusSymbols
	handler := a.screen.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone), nil)
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(84, 24)
	a.screen.SetRect(0, 0, 84, 24)
	a.screen.Draw(sim)

	l := calculateLayout(84, 24)
	cell, style, _ := sim.Get(l.card.x+2, l.card.y+6)
	foreground, _, _ := style.Decompose()
	if cell != "L" || foreground != a.screen.colors.focus {
		t.Fatalf("length label = %q with color %v, want focused Length", cell, foreground)
	}
}

func TestSliderPointerClampsAndSetsLength(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	l := calculateLayout(84, 24)
	railStart, railWidth := sliderRail(l.length)
	a.screen.setLengthFromPointer(l.length, 0, 0, railStart+railWidth-1)
	if got := a.CurrentOptions().Length; got != 256 {
		t.Fatalf("right rail endpoint got %d, want 256", got)
	}
	a.screen.setLengthFromPointer(l.length, 0, 0, railStart-20)
	if got := a.CurrentOptions().Length; got != 4 {
		t.Fatalf("left of rail got %d, want 4", got)
	}
}

func TestDrawPaintsEntireViewport(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	if err := a.screen.regenerate(); err != nil {
		t.Fatal(err)
	}
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(84, 24)
	a.screen.SetRect(0, 0, 84, 24)
	a.screen.Draw(sim)

	_, style, _ := sim.Get(83, 23)
	foreground, background, _ := style.Decompose()
	if background != a.screen.colors.base {
		t.Fatalf("viewport background = %v, want %v (foreground %v)", background, a.screen.colors.base, foreground)
	}
	logo := calculateLogoLayout(84, 24)
	if logo.style != logoWordmark {
		t.Fatalf("84x24 logo style = %d, want wordmark", logo.style)
	}
	if logo.x != (84-logo.w)/2 {
		t.Fatalf("logo x = %d, want centered %d", logo.x, (84-logo.w)/2)
	}
	wordmark, _, _ := sim.Get(logo.x, 0)
	if wordmark != "l" {
		t.Fatalf("logo wordmark was not rendered, got %q", wordmark)
	}
}

func TestDrawRendersLiteralShortcutBadges(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	if err := a.screen.regenerate(); err != nil {
		t.Fatal(err)
	}
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(84, 24)
	a.screen.SetRect(0, 0, 84, 24)
	a.screen.Draw(sim)

	l := calculateLayout(84, 24)
	badgeX := l.upper.x + 18
	cell, _, _ := sim.Get(badgeX, l.upper.y)
	if cell != "[" {
		t.Fatalf("shortcut badge starts with %q, want '['", cell)
	}
}

func TestCompactAmbigHidesExample(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil)
	if err := a.screen.regenerate(); err != nil {
		t.Fatal(err)
	}
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(50, 18)
	a.screen.SetRect(0, 0, 50, 18)
	a.screen.Draw(sim)

	l := calculateLayout(50, 18)
	if !l.compact {
		t.Fatalf("50x18 should use compact layout: %+v", l)
	}
	var row strings.Builder
	for xx := l.ambig.x; xx < l.ambig.x+l.ambig.w; xx++ {
		cell, _, _ := sim.Get(xx, l.ambig.y)
		row.WriteString(cell)
	}
	if !strings.HasPrefix(strings.TrimSpace(row.String()), "[ ] Exclude ambigu") {
		t.Fatalf("compact ambig row missing full label, got %q", row.String())
	}
	if strings.Contains(row.String(), "(I") {
		t.Fatalf("compact ambig row should hide the example, got %q", row.String())
	}
}

func TestToggleDoesNotShiftBadge(t *testing.T) {
	ambigRow := func(exclude bool) string {
		c := config.Defaults()
		c.ExcludeAmbiguous = exclude
		a := NewApp(c, "", nil)
		if err := a.screen.regenerate(); err != nil {
			t.Fatal(err)
		}
		sim := tcell.NewSimulationScreen("UTF-8")
		if err := sim.Init(); err != nil {
			t.Fatal(err)
		}
		defer sim.Fini()
		sim.SetSize(84, 24)
		a.screen.SetRect(0, 0, 84, 24)
		a.screen.Draw(sim)

		l := calculateLayout(84, 24)
		var row strings.Builder
		for xx := l.ambig.x; xx < l.ambig.x+l.ambig.w; xx++ {
			cell, _, _ := sim.Get(xx, l.ambig.y)
			row.WriteString(cell)
		}
		return row.String()
	}

	off, on := ambigRow(false), ambigRow(true)
	badgeCell := func(row string) int {
		return len([]rune(row[:strings.Index(row, "[E]")]))
	}
	if badgeCell(off) != badgeCell(on) {
		t.Fatalf("toggling E moved its badge: %q vs %q", off, on)
	}
	if !strings.Contains(on, "(I l 1 O 0)") || strings.Index(on, "(I l 1 O 0)") > strings.Index(on, "[E]") {
		t.Fatalf("example should sit before the badge, got %q", on)
	}
}

func TestRouteSwitchPreservesGeneratorState(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil, WithInitialRoute(VaultRoute))
	if a.CurrentRoute() != VaultRoute {
		t.Fatal("initial route should be vault")
	}
	a.screen.switchRoute()
	a.screen.selected = focusSymbols
	a.screen.switchRoute()
	a.screen.switchRoute()
	if a.CurrentRoute() != GeneratorRoute || a.screen.selected != focusSymbols {
		t.Fatal("generator state should survive route switching")
	}
}

func TestVaultBrowseAndCopy(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"":     {{Name: "team", Path: vault.Path{"team"}, Kind: vault.FolderNode}},
		"team": {{Name: "site", Path: vault.Path{"team", "site"}, Kind: vault.EntryNode}},
	}, read: map[string]vault.Entry{"team/site": {Password: "secret"}}}
	var copied string
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.openVaultNode()
	if got := a.screen.vaultPath.String(); got != "team" {
		t.Fatalf("path = %q", got)
	}
	a.screen.openVaultNode()
	if copied != "secret" || a.screen.status != "Copied to clipboard" || a.screen.statusLevel != notificationSuccess {
		t.Fatalf("copy = %q, notification = %q/%d", copied, a.screen.status, a.screen.statusLevel)
	}
}

func TestCopyConfirmationDoesNotBlockVaultNavigation(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"": {{Name: "first", Path: vault.Path{"first"}, Kind: vault.EntryNode}, {Name: "second", Path: vault.Path{"second"}, Kind: vault.EntryNode}},
	}, read: map[string]vault.Entry{"first": {Password: "one"}, "second": {Password: "two"}}}
	var copied string
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.copyVaultEntry()
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	a.screen.copyVaultEntry()
	if copied != "two" || a.screen.vaultSelected != 1 {
		t.Fatalf("copy=%q selected=%d", copied, a.screen.vaultSelected)
	}
}

func TestVaultVimNavigation(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"":     {{Name: "alpha", Path: vault.Path{"alpha"}, Kind: vault.FolderNode}, {Name: "team", Path: vault.Path{"team"}, Kind: vault.FolderNode}},
		"team": {{Name: "site", Path: vault.Path{"team", "site"}, Kind: vault.EntryNode}},
	}, read: map[string]vault.Entry{"team/site": {Password: "secret"}}}
	var copied string
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.vaultSelected = 1
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone))
	if got := a.screen.vaultPath.String(); got != "team" {
		t.Fatalf("l should open folder, path = %q", got)
	}
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone))
	if copied != "secret" {
		t.Fatal("l should copy the selected entry")
	}
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone))
	if len(a.screen.vaultPath) != 0 || a.screen.vaultSelected != 1 {
		t.Fatalf("h should restore the parent selection, path = %q selected = %d", a.screen.vaultPath, a.screen.vaultSelected)
	}
}

func TestVaultFilterCombinesTypingAndPickerNavigation(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"": {{Name: "github", Path: vault.Path{"github"}, Kind: vault.EntryNode}, {Name: "gitlab", Path: vault.Path{"gitlab"}, Kind: vault.EntryNode}, {Name: "personal", Path: vault.Path{"personal"}, Kind: vault.FolderNode}},
	}, read: map[string]vault.Entry{"github": {Password: "one"}, "gitlab": {Password: "two"}}}
	var copied string
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	a.screen.handleVaultFilterInput(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))
	if !a.screen.vaultFiltering || len(a.screen.filteredVaultNodes()) != 2 {
		t.Fatalf("filter=%q nodes=%#v", a.screen.vaultFilter, a.screen.filteredVaultNodes())
	}
	a.screen.handleVaultFilterInput(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	a.screen.handleVaultFilterInput(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if copied != "two" {
		t.Fatalf("picker copied %q", copied)
	}
	a.screen.handleVaultFilterInput(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if a.screen.vaultFiltering || a.screen.vaultFilter != "" {
		t.Fatal("escape should close and clear the filter")
	}
}

func TestStoreFormCancelFailureAndSuccess(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{}}
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p}))
	a.screen.password = "generated"
	a.screen.openStoreForm()
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if a.screen.storeForm {
		t.Fatal("escape should cancel store form")
	}
	p.err = errors.New("write failed")
	a.screen.openStoreForm()
	a.screen.storePath = "folder/name"
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !a.screen.storeForm || a.screen.status != "Store failed" {
		t.Fatalf("form=%t status=%q", a.screen.storeForm, a.screen.status)
	}
	p.err = nil
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if p.store.Path.String() != "folder/name" || p.store.Password != "generated" || a.screen.storeForm {
		t.Fatal("generated password was not stored")
	}
}

func TestStoreActionOpensStoreForm(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: &memoryVault{}}))
	a.screen.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone), nil)
	if !a.screen.storeForm {
		t.Fatal("a should open the Add to vault form")
	}
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	wasSymbols := a.screen.cfg.Symbols
	a.screen.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone), nil)
	if a.screen.cfg.Symbols == wasSymbols {
		t.Fatal("s should toggle Symbols")
	}
}

func TestStoreFormSuggestsAndInsertsFolders(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"":         {{Name: "home", Path: vault.Path{"home"}, Kind: vault.FolderNode}, {Name: "personal", Path: vault.Path{"personal"}, Kind: vault.FolderNode}},
		"personal": {{Name: "email", Path: vault.Path{"personal", "email"}, Kind: vault.FolderNode}},
	}}
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p}))
	a.screen.openStoreForm()
	if len(a.screen.storeNodes) != 2 {
		t.Fatalf("root suggestions = %#v", a.screen.storeNodes)
	}
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModNone))
	if len(a.screen.storeNodes) != 1 || a.screen.storeNodes[0].Name != "personal" {
		t.Fatalf("prefix suggestions = %#v", a.screen.storeNodes)
	}
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyTAB, 0, tcell.ModNone))
	if a.screen.storePath != "personal/" || len(a.screen.storeNodes) != 1 || a.screen.storeNodes[0].Name != "email" {
		t.Fatalf("path=%q suggestions=%#v", a.screen.storePath, a.screen.storeNodes)
	}
}

func TestStoreFormDoesNotWarnForMissingFolder(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: &memoryVault{err: vault.ErrNotFound}}))
	a.screen.openStoreForm()
	if a.screen.storeMessage != "" {
		t.Fatalf("missing folder message = %q", a.screen.storeMessage)
	}
}

func TestStoreSuggestionQuery(t *testing.T) {
	for _, test := range []struct {
		destination string
		path        string
		prefix      string
	}{
		{"", "", ""},
		{"ho", "", "ho"},
		{"Personal/", "Personal", ""},
		{"Personal/e", "Personal", "e"},
	} {
		path, prefix, err := storeSuggestionQuery(test.destination)
		if err != nil || path.String() != test.path || prefix != test.prefix {
			t.Fatalf("query(%q) = (%q, %q, %v)", test.destination, path, prefix, err)
		}
	}
}

func TestInputWordControls(t *testing.T) {
	if got := deletePreviousWord("Personal/Email"); got != "Personal/" {
		t.Fatalf("path Ctrl-W = %q", got)
	}
	if got := deletePreviousWord("two words"); got != "two" {
		t.Fatalf("text Ctrl-W = %q", got)
	}
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: &memoryVault{}}))
	a.screen.openStoreForm()
	a.screen.storePath = "Personal/Email"
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone))
	if a.screen.storePath != "Personal/" {
		t.Fatalf("store Ctrl-W = %q", a.screen.storePath)
	}
	a.screen.handleStoreInput(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModNone))
	if a.screen.storePath != "" {
		t.Fatalf("store Ctrl-U = %q", a.screen.storePath)
	}
	a.screen.vaultFiltering, a.screen.vaultFilter = true, "github"
	a.screen.handleVaultFilterInput(tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone))
	if a.screen.vaultFilter != "" {
		t.Fatalf("filter Ctrl-W = %q", a.screen.vaultFilter)
	}
}

func TestStoreFormHelpFitsAvailableWidth(t *testing.T) {
	for _, width := range []int{48, 38, 28, 18} {
		if got := storeFormHelp(width); uniseg.StringWidth(got) > width {
			t.Fatalf("help %q exceeds width %d", got, width)
		}
	}
}

func TestVaultNodeLabelsAndColors(t *testing.T) {
	folder := vault.Node{Name: "personal", Kind: vault.FolderNode}
	entry := vault.Node{Name: "proton", Kind: vault.EntryNode}
	a := NewApp(config.Defaults(), "", nil)
	if got := a.screen.vaultNodeLabel(folder); got != " personal/" {
		t.Fatalf("default folder label = %q", got)
	}
	if got := a.screen.vaultNodeLabel(entry); got != "󰦝 proton" {
		t.Fatalf("default entry label = %q", got)
	}
	if a.screen.vaultNodeColor(folder) != a.screen.colors.accent || a.screen.vaultNodeColor(entry) != a.screen.colors.text {
		t.Fatal("vault node colors should distinguish folders from entries")
	}
	cfg := config.Defaults()
	cfg.NerdFont = false
	a = NewApp(cfg, "", nil)
	if got := a.screen.vaultNodeLabel(folder); got != "personal/" {
		t.Fatalf("ASCII folder label = %q", got)
	}
	if got := a.screen.vaultNodeLabel(entry); got != "proton" {
		t.Fatalf("ASCII entry label = %q", got)
	}
}

func TestVaultMouseSelectsAndActivates(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"":     {{Name: "alpha", Path: vault.Path{"alpha"}, Kind: vault.FolderNode}, {Name: "team", Path: vault.Path{"team"}, Kind: vault.FolderNode}},
		"team": {{Name: "site", Path: vault.Path{"team", "site"}, Kind: vault.EntryNode}},
	}, read: map[string]vault.Entry{"team/site": {Password: "secret"}}}
	var copied string
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p, Copy: func(password string) error { copied = password; return nil }}), WithInitialRoute(VaultRoute))
	if err := a.Init(); err != nil {
		t.Fatal(err)
	}
	a.screen.SetRect(0, 0, 80, 24)
	l := calculateLayout(80, 24)
	r := l.card
	listY := r.y + 2
	at := func(row int) *tcell.EventMouse {
		return tcell.NewEventMouse(r.x+2, listY+row, tcell.Button1, tcell.ModNone)
	}
	if a.screen.vaultSelected != 0 {
		t.Fatalf("initial selected = %d", a.screen.vaultSelected)
	}
	a.screen.handleVaultMouse(tview.MouseLeftClick, at(1), l, 0, 0)
	if a.screen.vaultSelected != 1 {
		t.Fatalf("click should select row, selected = %d", a.screen.vaultSelected)
	}
	if got := a.screen.vaultPath.String(); got != "" {
		t.Fatalf("first click should not open folder, path = %q", got)
	}
	a.screen.handleVaultMouse(tview.MouseLeftClick, at(1), l, 0, 0)
	if got := a.screen.vaultPath.String(); got != "team" {
		t.Fatalf("second click should open folder, path = %q", got)
	}
	a.screen.handleVaultMouse(tview.MouseLeftDoubleClick, at(1), l, 0, 0)
	if copied != "secret" {
		t.Fatalf("double-click should copy entry, copied = %q", copied)
	}
	a.screen.handleVaultMouse(tview.MouseScrollDown, at(0), l, 0, 0)
	a.screen.handleVaultMouse(tview.MouseScrollUp, at(0), l, 0, 0)
	a.screen.handleVaultMouse(tview.MouseLeftClick, at(0), l, 0, 0)
	a.screen.handleVaultMouse(tview.MouseLeftClick, at(0), l, 0, 0)
	if got := a.screen.vaultPath.String(); got != "" {
		t.Fatalf("../ clicks should go to parent, path = %q", got)
	}
	a.screen.handleVaultMouse(tview.MouseLeftClick, at(1), l, 0, 0)
	a.screen.handleVaultMouse(tview.MouseLeftClick, at(1), l, 0, 0)
	a.screen.handleVaultMouse(tview.MouseRightClick, at(0), l, 0, 0)
	if got := a.screen.vaultPath.String(); got != "" {
		t.Fatalf("right-click should go to parent, path = %q", got)
	}
}

func TestVaultThemeShortcut(t *testing.T) {
	a := NewApp(config.Defaults(), "", nil, WithInitialRoute(VaultRoute))
	a.screen.handleVaultInput(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
	if !a.screen.themePanel.open {
		t.Fatal("t should open the theme picker in vault view")
	}
}

func TestStorePickerMouseSelectsAndCompletesFolder(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{
		"": {{Name: "home", Path: vault.Path{"home"}, Kind: vault.FolderNode}, {Name: "personal", Path: vault.Path{"personal"}, Kind: vault.FolderNode}},
	}}
	a := NewApp(config.Defaults(), "", nil, WithVault(service.Service{Provider: p}))
	a.screen.SetRect(0, 0, 80, 24)
	a.screen.openStoreForm()
	r, _ := a.screen.storeFormRect(0, 0, 80, 24)
	event := tcell.NewEventMouse(r.x+2, r.y+7, tcell.Button1, tcell.ModNone)
	a.screen.handleStoreMouse(tview.MouseLeftDown, event)
	if a.screen.storeSelected != 1 {
		t.Fatalf("selected = %d", a.screen.storeSelected)
	}
	a.screen.handleStoreMouse(tview.MouseLeftClick, event)
	if a.screen.storePath != "personal/" {
		t.Fatalf("path = %q", a.screen.storePath)
	}
}
