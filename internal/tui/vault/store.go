package vault

import (
	"context"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

type StoreDependencies struct {
	RunOperation OperationRunner
	Service      service.Service
	Password     func() string
	Notify       func(shared.Level, string)
	Closed       func()
	RunPrompted  PromptRunner
	Saved        func(vault.Path)
}

type StoreForm struct {
	saving               bool
	open                 bool
	storePath            string
	storeNodes           []vault.Node
	storeSelected        int
	storeMessage         string
	deps                 StoreDependencies
	manual               bool
	editingValue         bool
	value                string
	editEntry            *vault.Entry
	suggestions          map[string][]vault.Node
	suggestionGeneration uint64
	suggestionPending    map[string]bool
}

func NewStoreForm(deps StoreDependencies) *StoreForm { return &StoreForm{deps: deps} }
func (s *StoreForm) IsOpen() bool                    { return s.open }
func (s *StoreForm) notify(level shared.Level, message string) {
	if s.deps.Notify != nil {
		s.deps.Notify(level, message)
	}
}

func (s *StoreForm) run(work func(context.Context) error, done func(error)) {
	s.saving = true
	generation := s.suggestionGeneration
	s.deps.RunOperation.Execute(s.deps.RunPrompted, work, func(err error) {
		if generation != s.suggestionGeneration || !s.open {
			return
		}
		s.saving = false
		done(err)
	})
}

func (s *StoreForm) saved(path vault.Path, message string) {
	if s.deps.Saved != nil {
		s.deps.Saved(path)
	}
	s.Close()
	s.notify(shared.Success, message)
}
func (s *StoreForm) Open() bool {
	if s.deps.Service.Provider == nil {
		s.notify(shared.Warning, "Vault unavailable")
		return false
	}
	*s = StoreForm{deps: s.deps, open: true, suggestions: make(map[string][]vault.Node), suggestionPending: make(map[string]bool), suggestionGeneration: s.suggestionGeneration + 1}
	s.refreshStoreSuggestions()
	return true
}

func (s *StoreForm) OpenAdd(parent vault.Path) bool {
	if !s.Open() {
		return false
	}
	s.manual = true
	if len(parent) > 0 {
		s.storePath = parent.String() + "/"
		s.refreshStoreSuggestions()
	}
	return true
}
func (s *StoreForm) Close() {
	*s = StoreForm{deps: s.deps, suggestionGeneration: s.suggestionGeneration + 1}
	if s.deps.Closed != nil {
		s.deps.Closed()
	}
}

func (s *StoreForm) OpenEdit(path vault.Path) bool {
	*s = StoreForm{deps: s.deps, open: true, manual: true, editingValue: true, saving: true, storePath: path.String(), suggestionGeneration: s.suggestionGeneration + 1}
	var entry vault.Entry
	provider := s.deps.Service
	s.run(func(ctx context.Context) error {
		var err error
		entry, err = provider.Show(ctx, path)
		return err
	}, func(err error) {
		if err != nil {
			s.notify(shared.Error, copyErrorMessage(err))
			s.Close()
			return
		}
		s.storePath = path.String()
		s.value = entry.Password
		entry.Password = ""
		entry.Path = append(vault.Path(nil), path...)
		s.editEntry = &entry
	})
	return s.open
}
