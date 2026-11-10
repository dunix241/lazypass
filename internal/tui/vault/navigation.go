package vault

import (
	"context"
	"errors"

	"lazypass/internal/debug"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
)

func (s *View) Refresh() {
	s.refresh("")
}

func (s *View) refresh(selectPath string) {
	if s.cacheLoading {
		return
	}
	if s.loadBrowserCache(selectPath) {
		return
	}
	if len(s.vaultNodes) == 0 {
		s.restoreCachedLocation()
	}
	selectionRestored := s.selectNodePath(selectPath)
	s.vaultError = ""
	s.loadGeneration++
	generation := s.loadGeneration
	path := append(vault.Path(nil), s.vaultPath...)
	initialSelection := s.vaultSelected
	_, hasCachedLocation := s.cachedLocations[path.String()]
	s.loading = len(s.vaultNodes) == 0 && !hasCachedLocation
	s.refreshing = !s.loading
	var nodes []vault.Node
	s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error {
		var err error
		nodes, err = s.vault.List(ctx, path)
		if err == nil && s.deps.Cache != nil {
			if cacheErr := s.deps.Cache.Save(path, nodes); cacheErr != nil {
				debug.Failure("browser cache save", debug.IO)
			}
		}
		return err
	}, func(err error) {
		if generation != s.loadGeneration {
			return
		}
		s.loading = false
		s.refreshing = false
		completion := s.refreshCompletion
		s.refreshCompletion = nil
		defer func() {
			if completion != nil {
				completion(err)
			}
		}()
		if err != nil {
			if s.vault.Provider == nil {
				return
			}
			debug.Failure("vault list", vaultFailureCause(err))
			message := "Vault unavailable or folder cannot be opened."
			var diagnostic vault.Diagnostic
			if errors.As(err, &diagnostic) {
				message = diagnostic.UserMessage()
			}
			var recovery vault.RecoveryCommand
			if s.deps.NotifyRecovery != nil && errors.As(err, &recovery) {
				s.deps.NotifyRecovery(message, recovery.RecoveryCommand())
				return
			}
			if len(s.vaultNodes) > 0 || hasCachedLocation {
				if diagnostic == nil {
					message = "Refresh failed; showing cached entries"
				}
				s.notify(shared.Warning, message)
				return
			}
			s.vaultError = message
			nodes = nil
		}
		if current, ok := s.selectedVaultNode(); ok && (selectionRestored || s.vaultSelected != initialSelection || selectPath == "") {
			selectPath = current.Path.String()
		}
		s.vaultNodes = nodes
		if err == nil {
			if s.cachedLocations == nil {
				s.cachedLocations = make(map[string][]vault.Node)
			}
			s.cachedLocations[path.String()] = nodes
			s.rememberLocationParents(path, nodes)
		}
		s.rememberLocationLabels(nodes)
		if s.vaultFiltering {
			s.loadSearchNodes()
		} else {
			s.CancelSearch()
			s.searchNodes = nil
			s.searchLoaded = false
		}
		s.vaultSelected = min(s.vaultSelected, max(0, len(s.vaultDisplayNodes())-1))
		s.selectNodePath(selectPath)
	})
}

func (s *View) selectNodePath(path string) bool {
	if path == "" {
		return false
	}
	for i, node := range s.vaultDisplayNodes() {
		if node.Path.String() == path {
			s.vaultSelected = i
			return true
		}
	}
	return false
}

func (s *View) restoreCachedLocation() {
	if nodes, ok := s.cachedLocations[s.vaultPath.String()]; ok {
		s.vaultNodes = nodes
	}
}

func (s *View) rememberLocationLabels(nodes []vault.Node) {
	if s.locationLabels == nil {
		s.locationLabels = make(map[string]string)
	}
	for _, node := range nodes {
		if node.Kind == vault.FolderNode {
			s.locationLabels[node.Path.String()] = node.Name
		}
	}
}

func (s *View) loadBrowserCache(selectPath string) bool {
	if s.cacheLoaded || s.deps.Cache == nil || s.deps.Context == nil || s.deps.Dispatch == nil {
		return false
	}
	ctx := s.deps.Context()
	if ctx == nil || ctx.Err() != nil {
		return false
	}
	s.cacheLoaded, s.cacheLoading = true, true
	s.loading = len(s.vaultNodes) == 0
	go func() {
		cache := s.deps.Cache
		var err error
		if identity, ok := s.vault.Provider.(vault.BrowserCacheIdentity); ok {
			var account string
			account, err = identity.BrowserCacheIdentity(ctx)
			if err == nil {
				cache = cache.Scoped(account)
			} else {
				cache = nil
			}
		}
		var locations map[string][]vault.Node
		if err == nil {
			locations, err = cache.Load()
		}
		s.deps.Dispatch(ctx, func() {
			s.cacheLoading = false
			s.deps.Cache = cache
			if err == nil {
				s.cachedLocations = locations
				for parent, nodes := range locations {
					s.rememberLocationLabels(nodes)
					if path, err := vault.ParsePath(parent); err == nil {
						s.rememberLocationParents(path, nodes)
					}
				}
				s.restoreCachedLocation()
			} else {
				debug.Failure("browser cache load", debug.IO)
			}
			s.refresh(selectPath)
		})
	}()
	return true
}

func (s *View) navigate(path vault.Path, selected int, selectPath string) {
	s.CancelSearch()
	s.searchNodes, s.searchLoaded = nil, false
	s.vaultFiltering, s.vaultFilter = false, ""
	s.vaultPath = append(vault.Path(nil), path...)
	s.vaultSelected = selected
	s.vaultNodes = nil
	s.refresh(selectPath)
}
func (s *View) moveVaultSelection(delta int) {
	nodes := s.vaultDisplayNodes()
	s.vaultSelected = min(max(0, len(nodes)-1), max(0, s.vaultSelected+delta))
}
func (s *View) vaultParent() {
	if len(s.vaultPath) == 0 {
		return
	}
	child := s.vaultPath.String()
	s.navigate(s.parentPath(), 0, child)
}

func (s *View) rememberLocationParents(parent vault.Path, nodes []vault.Node) {
	if s.locationParents == nil {
		s.locationParents = make(map[string]vault.Path)
	}
	for _, node := range nodes {
		if node.Kind == vault.FolderNode && node.Path.String() != parent.String() {
			s.locationParents[node.Path.String()] = append(vault.Path(nil), parent...)
		}
	}
}

func (s *View) parentPath() vault.Path {
	if parent, ok := s.locationParents[s.vaultPath.String()]; ok {
		return append(vault.Path(nil), parent...)
	}
	if len(s.vaultPath) == 0 {
		return nil
	}
	return append(vault.Path(nil), s.vaultPath[:len(s.vaultPath)-1]...)
}
func (s *View) openVaultNode() {
	node, ok := s.selectedVaultNode()
	if !ok {
		return
	}
	if s.isVaultParentNode(node) {
		s.vaultParent()
		return
	}
	if node.Kind == vault.FolderNode {
		s.navigate(node.Path, 1, "")
		return
	}
	s.copyVaultEntry()
}
func (s *View) copyVaultEntry() {
	s.copyEntry(false)
}

func (s *View) copyEntry(authenticator bool) {
	node, ok := s.selectedVaultNode()
	if !ok || node.Kind != vault.EntryNode {
		return
	}
	if s.copying {
		return
	}
	s.copying = true
	copyContext, cancel := context.WithCancel(context.Background())
	s.copyCancel = cancel
	s.progress("Copying…")
	provider := s.vault
	s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error {
		ctx, stop := context.WithCancel(ctx)
		defer stop()
		unregister := context.AfterFunc(copyContext, stop)
		defer unregister()
		if copyContext.Err() != nil {
			return context.Canceled
		}
		if authenticator {
			return provider.CopyAuthenticator(ctx, node.Path)
		}
		return provider.CopyPassword(ctx, node.Path, copyContext)
	}, func(err error) {
		cancel()
		s.copyCancel = nil
		s.copying = false
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			var diagnostic vault.Diagnostic
			if !errors.As(err, &diagnostic) {
				debug.Failure("vault copy", vaultFailureCause(err))
			}
			s.notify(shared.Error, copyErrorMessage(err))
			return
		}
		s.notify(shared.Success, "Copied to clipboard")
	})
}
func (s *View) selectedVaultNode() (vault.Node, bool) {
	nodes := s.vaultDisplayNodes()
	if s.vaultSelected < 0 || s.vaultSelected >= len(nodes) {
		return vault.Node{}, false
	}
	return nodes[s.vaultSelected], true
}
