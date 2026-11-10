package vault

import (
	"context"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
	"lazypass/internal/vault/browsercache"
	"lazypass/internal/vault/service"
)

type Dependencies struct {
	Progress       func(string)
	NotifyRecovery func(string, string)
	Context        func() context.Context
	Cache          *browsercache.Store
	RunOperation   OperationRunner
	Name           string
	Service        service.Service
	RunPrompted    PromptRunner
	Notify         func(shared.Level, string)
	OpenThemes     func()
	OpenAdd        func()
	OpenEdit       func(vault.Path)
	Dispatch       func(context.Context, func())
}

func (s *View) CurrentPath() vault.Path { return append(vault.Path(nil), s.vaultPath...) }

type View struct {
	refreshCompletion func(error)
	deletePlan        *vault.DeletePlan
	deleteRecursive   bool
	deleteDialog      *shared.Confirmation
	deleteGeneration  uint64
	locationParents   map[string]vault.Path
	cachedLocations   map[string][]vault.Node
	cacheLoaded       bool
	cacheLoading      bool
	syncing           bool
	locationLabels    map[string]string
	metadataLoading   bool
	refreshing        bool
	deleting          bool
	loading           bool
	copying           bool
	loadGeneration    uint64
	vault             service.Service
	vaultPath         vault.Path
	vaultNodes        []vault.Node
	vaultSelected     int
	vaultError        string
	vaultFilter       string
	vaultFiltering    bool
	searchNodes       []vault.Node
	searchLoaded      bool
	searchCancel      context.CancelFunc
	searchGeneration  uint64
	searching         bool
	deps              Dependencies
	deleteTarget      *vault.Node
}

func (s *View) IsConfirmingDelete() bool { return s.deleteTarget != nil }

func New(deps Dependencies) *View { return &View{vault: deps.Service, deps: deps} }
func (s *View) IsFiltering() bool { return s.vaultFiltering }

func (s *View) notify(level shared.Level, message string) {
	if s.deps.Notify != nil {
		s.deps.Notify(level, message)
	}
}
