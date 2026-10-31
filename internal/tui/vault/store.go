package vault

import (
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

type StoreDependencies struct {
	Service  service.Service
	Password func() string
	Notify   func(shared.Level, string)
	Closed   func()
}

type StoreForm struct {
	open          bool
	storePath     string
	storeNodes    []vault.Node
	storeSelected int
	storeMessage  string
	deps          StoreDependencies
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
	s.open, s.storePath, s.storeSelected, s.storeMessage = true, "", 0, ""
	s.refreshStoreSuggestions()
	return true
}
func (s *StoreForm) Close() {
	s.open, s.storePath, s.storeNodes, s.storeSelected, s.storeMessage = false, "", nil, 0, ""
	if s.deps.Closed != nil {
		s.deps.Closed()
	}
}
