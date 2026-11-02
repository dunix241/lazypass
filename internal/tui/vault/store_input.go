package vault

import (
	"context"
	"errors"
	"strings"

	"lazypass/internal/debug"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

func (s *StoreForm) HandleKey(event *tcell.EventKey) {
	if s.saving {
		return
	}
	if s.editingValue {
		s.handleValueKey(event)
		return
	}
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
		s.storePath = shared.DeleteLastRune(s.storePath)
		s.refreshStoreSuggestions()
	case tcell.KeyEnter:
		if s.manual {
			if strings.HasSuffix(s.storePath, "/") {
				s.createFolder()
				return
			}
			path, err := vault.ParsePath(s.storePath)
			if err != nil || len(path) == 0 {
				s.notify(shared.Warning, "Enter a valid vault destination")
				return
			}
			provider := s.deps.Service
			s.run(func(ctx context.Context) error { return provider.CheckNewPath(ctx, path) }, func(err error) {
				if err != nil {
					s.notify(shared.Warning, storeErrorMessage(err))
					return
				}
				s.editingValue = true
				s.storeMessage = ""
			})
			return
		}
		s.store()
	case tcell.KeyRune:
		if event.Rune() >= ' ' {
			s.storePath += string(event.Rune())
			s.refreshStoreSuggestions()
		}
	}
}

func (s *StoreForm) store() {
	path, err := vault.ParsePath(s.storePath)
	if err != nil || len(path) == 0 {
		s.notify(shared.Warning, "Enter a valid vault destination")
		return
	}
	password := ""
	if s.manual {
		password = s.value
	} else if s.deps.Password != nil {
		password = s.deps.Password()
	}
	entry := vault.Entry{Path: path, Password: password}
	editing := s.editEntry != nil
	if editing {
		entry = *s.editEntry
		entry.Password = password
	}
	s.notify(shared.Info, "Saving…")
	provider := s.deps.Service
	s.run(func(ctx context.Context) error {
		if editing {
			return provider.Store(ctx, entry)
		}
		return provider.Create(ctx, entry)
	}, func(err error) {
		if err != nil {
			var diagnostic vault.Diagnostic
			if !errors.As(err, &diagnostic) {
				debug.Failure("vault store", vaultFailureCause(err))
			}
			s.notify(shared.Error, storeErrorMessage(err))
			return
		}
		s.saved(path, "Stored in vault")
	})
}

func (s *StoreForm) createFolder() {
	path, err := vault.ParsePath(strings.TrimSuffix(s.storePath, "/"))
	if err != nil || len(path) == 0 {
		s.notify(shared.Warning, "Enter a valid folder destination")
		return
	}
	s.notify(shared.Info, "Creating folder…")
	provider := s.deps.Service
	s.run(func(ctx context.Context) error { return provider.CreateFolder(ctx, path) }, func(err error) {
		if err != nil {
			s.notify(shared.Error, storeErrorMessage(err))
			return
		}
		s.saved(path, "Folder created")
	})
}

func (s *StoreForm) handleValueKey(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		s.Close()
	case tcell.KeyBacktab:
		if s.editEntry != nil {
			return
		}
		s.value = ""
		s.editingValue = false
	case tcell.KeyCtrlU:
		s.value = ""
	case tcell.KeyCtrlW:
		s.value = shared.DeletePreviousWord(s.value)
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		s.value = shared.DeleteLastRune(s.value)
	case tcell.KeyEnter:
		if s.value == "" {
			s.notify(shared.Warning, "Enter a value")
			return
		}
		s.store()
	case tcell.KeyRune:
		if event.Rune() >= ' ' {
			if len(s.value)+len(string(event.Rune())) > 64*1024 {
				s.notify(shared.Warning, "Value limit is 64 KiB")
				return
			}
			s.value += string(event.Rune())
		}
	}
}
