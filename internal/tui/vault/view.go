package vault

import (
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

type Dependencies struct {
	Service     service.Service
	RunPrompted func(func() error) error
	Notify      func(shared.Level, string)
	OpenThemes  func()
}

type View struct {
	vault          service.Service
	vaultPath      vault.Path
	vaultNodes     []vault.Node
	vaultSelected  int
	vaultError     string
	vaultFilter    string
	vaultFiltering bool
	deps           Dependencies
}

func New(deps Dependencies) *View { return &View{vault: deps.Service, deps: deps} }
func (s *View) IsFiltering() bool { return s.vaultFiltering }
func (s *View) notify(level shared.Level, message string) {
	if s.deps.Notify != nil {
		s.deps.Notify(level, message)
	}
}
