package vault

import (
	"context"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

type StoreDependencies struct {
	Service     service.Service
	Password    func() string
	Notify      func(shared.Level, string)
	Closed      func()
	RunPrompted PromptRunner
	Saved       func(vault.Path)
}

type StoreForm struct {
	open          bool
	storePath     string
	storeNodes    []vault.Node
	storeSelected int
	storeMessage  string
	deps          StoreDependencies
	manual        bool
	editingValue  bool
	value         string
	editEntry     *vault.Entry
}

func NewStoreForm(deps StoreDependencies) *StoreForm { return &StoreForm{deps: deps} }
func (s *StoreForm) IsOpen() bool                    { return s.open }
func (s *StoreForm) notify(level shared.Level, message string) {
	if s.deps.Notify != nil {
		s.deps.Notify(level, message)
	}
}
func (s *StoreForm) Open() bool {
	if s.deps.Service.Provider == nil {
		s.notify(shared.Warning, "Vault unavailable")
		return false
	}
	*s = StoreForm{deps: s.deps, open: true}
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
	*s = StoreForm{deps: s.deps}
	if s.deps.Closed != nil {
		s.deps.Closed()
	}
}

func (s *StoreForm) OpenEdit(path vault.Path) bool {
	var entry vault.Entry
	if err := s.deps.RunPrompted.Execute(func(ctx context.Context) error {
		var err error
		entry, err = s.deps.Service.Show(ctx, path)
		return err
	}); err != nil {
		s.notify(shared.Error, copyErrorMessage(err))
		return false
	}
	*s = StoreForm{deps: s.deps, open: true, manual: true, editingValue: true}
	s.storePath = path.String()
	s.value = entry.Password
	entry.Password = ""
	entry.Path = append(vault.Path(nil), path...)
	s.editEntry = &entry
	return true
}
