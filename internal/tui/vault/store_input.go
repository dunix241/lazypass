package vault

import (
	"context"
	"errors"

	"lazypass/internal/debug"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

func (s *StoreForm) HandleKey(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		s.Close()
	case tcell.KeyCtrlU:
		s.storePath = ""
		s.refreshStoreSuggestions()
	case tcell.KeyCtrlW:
		s.storePath = shared.DeletePreviousWord(s.storePath)
		s.refreshStoreSuggestions()
	case tcell.KeyUp:
		s.storeSelected = max(0, s.storeSelected-1)
	case tcell.KeyDown:
		s.storeSelected = min(max(0, len(s.storeNodes)-1), s.storeSelected+1)
	case tcell.KeyTAB:
		s.insertStoreSuggestion()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(s.storePath) > 0 {
			runes := []rune(s.storePath)
			s.storePath = string(runes[:len(runes)-1])
			s.refreshStoreSuggestions()
		}
	case tcell.KeyEnter:
		s.store()
	case tcell.KeyRune:
		if event.Rune() >= ' ' {
			s.storePath += string(event.Rune())
			s.refreshStoreSuggestions()
		}
	}
}

func (s *StoreForm) store() {
	path, err := vaultPath(s.storePath)
	if err != nil || len(path) == 0 {
		s.notify(shared.Warning, "Enter a valid vault destination")
		return
	}
	password := ""
	if s.deps.Password != nil {
		password = s.deps.Password()
	}
	if err := s.deps.Service.Store(context.Background(), vault.Entry{Path: path, Password: password}); err != nil {
		var diagnostic vault.Diagnostic
		if !errors.As(err, &diagnostic) {
			debug.Failure("vault store", vaultFailureCause(err))
		}
		s.notify(shared.Error, storeErrorMessage(err))
		return
	}
	s.Close()
	s.notify(shared.Success, "Stored in vault")
}
