package vault

import (
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"
)

type Dependencies struct {
	Service     service.Service
	RunPrompted PromptRunner
	Notify      func(shared.Level, string)
	OpenThemes  func()
	OpenAdd     func()
	OpenEdit    func(vault.Path)
}

func (s *View) CurrentPath() vault.Path { return append(vault.Path(nil), s.vaultPath...) }

type View struct {
	vault          service.Service
	vaultPath      vault.Path
	vaultNodes     []vault.Node
	vaultSelected  int
	vaultError     string
	vaultFilter    string
	vaultFiltering bool
	searchNodes    []vault.Node
	searchLoaded   bool
	deps           Dependencies
	deleteTarget   *vault.Node
}

func (s *View) IsConfirmingDelete() bool { return s.deleteTarget != nil }

func New(deps Dependencies) *View { return &View{vault: deps.Service, deps: deps} }
func (s *View) IsFiltering() bool { return s.vaultFiltering }
func (s *View) notify(level shared.Level, message string) {
	if s.deps.Notify != nil {
		s.deps.Notify(level, message)
	}
}
