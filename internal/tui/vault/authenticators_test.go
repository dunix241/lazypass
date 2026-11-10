package vault

import (
	"context"
	"testing"
	"time"

	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestInlineAuthenticatorLoadsAndClickCopiesCode(t *testing.T) {
	path := vault.Path{"entry"}
	p := &memoryVault{read: map[string]vault.Entry{"entry": {Password: "not-the-code", Authenticator: "JBSWY3DPEHPK3PXP"}}}
	updates := make(chan func(), 4)
	copied := ""
	v := New(Dependencies{
		Service:  service.Service{Provider: p, Copy: func(value string) error { copied = value; return nil }},
		Dispatch: func(_ context.Context, fn func()) { updates <- fn },
	})
	v.vaultNodes = []vault.Node{{Path: path, Name: "entry", Kind: vault.EntryNode}}
	frame := testFrame()
	v.authenticators.card = frame.Card
	v.TickAuthenticators(context.Background())
	select {
	case apply := <-updates:
		apply()
	case <-time.After(time.Second):
		t.Fatal("inline code did not load")
	}
	chip, code, remaining := v.authenticatorChip(v.vaultNodes[0], frame.Card, time.Now())
	if code == "" || remaining <= 0 || chip.W == 0 {
		t.Fatal("inline chip missing")
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	v.Draw(screen, frame)
	listY, _, _ := v.vaultListWindow(frame.Card)
	_, style, _ := screen.Get(chip.X, listY)
	_, background, _ := style.Decompose()
	if background != frame.Palette.Border {
		t.Fatal("code chip background missing")
	}
	v.HandleMouse(tview.MouseLeftClick, tcell.NewEventMouse(chip.X, listY, tcell.Button1, 0), frame)
	if len(copied) != 6 || copied == "not-the-code" {
		t.Fatal("chip click did not copy authenticator")
	}
	v.ClearAuthenticators()
	if _, code, _ := v.authenticatorChip(v.vaultNodes[0], frame.Card, time.Now()); code != "" {
		t.Fatal("cleared session retained displayed code")
	}
}

type protectedAuthenticatorProvider struct{ memoryVault }

func (p *protectedAuthenticatorProvider) Read(ctx context.Context, path vault.Path) (vault.Entry, error) {
	if vault.IsNoninteractive(ctx) {
		return vault.Entry{}, vault.ErrPromptRequired
	}
	return p.memoryVault.Read(ctx, path)
}

func TestInlineAuthenticatorStartsUnlockWithoutCopyAction(t *testing.T) {
	updates := make(chan func(), 4)
	prompts := 0
	v := New(Dependencies{
		Service: service.Service{Provider: &protectedAuthenticatorProvider{memoryVault{read: map[string]vault.Entry{
			"entry": {Authenticator: "JBSWY3DPEHPK3PXP"},
		}}}},
		Dispatch: func(_ context.Context, fn func()) { updates <- fn },
		RunOperation: func(work func(context.Context) error, done func(error)) {
			prompts++
			done(work(context.Background()))
		},
	})
	v.vaultNodes = []vault.Node{{Path: vault.Path{"entry"}, Kind: vault.EntryNode}}
	v.authenticators.card = shared.Rect{W: 80, H: 24}
	v.TickAuthenticators(context.Background())
	select {
	case apply := <-updates:
		apply()
	case <-time.After(time.Second):
		t.Fatal("unlock not requested")
	}
	row := v.authenticators.rows["entry"]
	if prompts != 1 || !row.loaded || row.value.Code == "" || v.authenticators.cancel != nil {
		t.Fatal("automatic unlock did not load inline code")
	}
}

func TestInlineAuthenticatorRejectsStaleLoadingResult(t *testing.T) {
	updates := make(chan func(), 4)
	v := New(Dependencies{
		Service:  service.Service{Provider: &memoryVault{read: map[string]vault.Entry{"entry": {Authenticator: "JBSWY3DPEHPK3PXP"}}}},
		Dispatch: func(_ context.Context, fn func()) { updates <- fn },
	})
	v.vaultNodes = []vault.Node{{Path: vault.Path{"entry"}, Kind: vault.EntryNode}}
	v.authenticators.card = shared.Rect{W: 80, H: 24}
	v.TickAuthenticators(context.Background())
	select {
	case apply := <-updates:
		v.ClearAuthenticators()
		apply()
	case <-time.After(time.Second):
		t.Fatal("worker did not finish")
	}
	if len(v.authenticators.rows) != 0 || v.authenticators.cancel != nil {
		t.Fatal("stale worker restored session")
	}
}

func TestAuthenticatorQueueHandlesSelectionBeyondShrinkingResults(t *testing.T) {
	updates := make(chan func(), 1)
	v := New(Dependencies{
		Service:  service.Service{Provider: &memoryVault{}},
		Dispatch: func(_ context.Context, fn func()) { updates <- fn },
	})
	v.vaultSelected = 100
	v.vaultNodes = []vault.Node{{Path: vault.Path{"folder"}, Kind: vault.FolderNode}}
	v.authenticators.card = shared.Rect{W: 80, H: 24}
	v.TickAuthenticators(context.Background())
	if v.authenticators.cancel != nil {
		t.Fatal("folder started secret loading")
	}
}
