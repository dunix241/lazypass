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
	if s.choosingType {
		switch event.Key() {
		case tcell.KeyEscape:
			s.Close()
		case tcell.KeyDown, tcell.KeyTAB:
			s.moveEntryType(1)
		case tcell.KeyUp, tcell.KeyBacktab:
			s.moveEntryType(-1)
		case tcell.KeyEnter:
			s.choosingType = false
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				s.moveEntryType(1)
			case 'k':
				s.moveEntryType(-1)
			default:
				s.choosingType = false
				s.HandleKey(event)
			}
		}
		return
	}
	if s.loadingEntries {
		if event.Key() == tcell.KeyEscape {
			s.Close()
		}
		return
	}
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
	case tcell.KeyUp:
		s.storeSelected = max(0, s.storeSelected-1)
	case tcell.KeyDown:
		s.storeSelected = min(max(0, len(s.storeNodes)-1), s.storeSelected+1)
	case tcell.KeyTAB:
		s.insertStoreSuggestion()
	case tcell.KeyEnter:
		{
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
				if !s.manual && s.deps.Password != nil {
					s.value = s.deps.Password()
				}
			})
			return
		}
	default:
		s.editInput(event)
	}
}

func (s *StoreForm) store() {
	s.commitField()
	path, err := vault.ParsePath(s.storePath)
	if err != nil || len(path) == 0 {
		s.notify(shared.Warning, "Enter a valid vault destination")
		return
	}
	password := s.value
	entry := vault.Entry{Path: path, Password: password}
	editing := s.editEntry != nil
	if editing {
		entry = *s.editEntry
		entry.Password = password
	}
	entry.Username, entry.URL, entry.Notes = s.username, s.url, s.notes
	entry.Type = s.entryType
	if s.extraFields != nil {
		entry.Fields = make(map[string]string, len(s.extraFields))
		for key, value := range s.extraFields {
			entry.Fields[key] = value
		}
	}
	provider := s.deps.Service
	s.run(func(ctx context.Context) error {
		if editing {
			return provider.Store(ctx, entry)
		}
		return provider.Create(ctx, entry)
	}, func(err error) {
		var warning vault.SavedWarning
		if errors.As(err, &warning) {
			s.saved(path, warning.UserMessage())
			return
		}
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
		if s.field > 0 {
			s.selectField(s.field - 1)
			return
		}
		if s.editEntry != nil {
			return
		}
		s.value = ""
		s.editingValue = false
	case tcell.KeyTAB:
		s.selectField((s.field + 1) % len(s.entryFields()))
	case tcell.KeyEnter:
		if s.entryType == vault.LoginEntry && s.value == "" {
			s.notify(shared.Warning, "Enter a value")
			return
		}
		s.store()
	default:
		s.editInput(event)
	}
}

func (s *StoreForm) moveEntryType(delta int) {
	types := s.deps.Service.SupportedEntryTypes()
	for index, kind := range types {
		if kind == s.entryType {
			s.entryType = types[(index+delta+len(types))%len(types)]
			return
		}
	}
}
