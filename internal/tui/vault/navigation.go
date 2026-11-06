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
	s.vaultError = ""
	s.loadGeneration++
	generation := s.loadGeneration
	path := append(vault.Path(nil), s.vaultPath...)
	selectedPath := ""
	if node, ok := s.selectedVaultNode(); ok {
		selectedPath = node.Path.String()
	}
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
		s.vaultNodes = nodes
		s.showingCached = false
		if err == nil {
			if s.cachedLocations == nil {
				s.cachedLocations = make(map[string][]vault.Node)
			}
			s.cachedLocations[path.String()] = nodes
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
		if selectPath == "" {
			selectPath = selectedPath
		}
		if selectPath != "" {
			for i, node := range s.vaultDisplayNodes() {
				if !s.isVaultParentNode(node) && node.Path.String() == selectPath {
					s.vaultSelected = i
					break
				}
			}
		}
	})
}

func (s *View) restoreCachedLocation() {
	if nodes, ok := s.cachedLocations[s.vaultPath.String()]; ok {
		s.vaultNodes = nodes
		s.showingCached = true
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
		locations, err := s.deps.Cache.Load()
		s.deps.Dispatch(ctx, func() {
			s.cacheLoading = false
			if err == nil {
				s.cachedLocations = locations
				for _, nodes := range locations {
					s.rememberLocationLabels(nodes)
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
	s.vaultNodes, s.showingCached = nil, false
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
	s.navigate(s.vaultPath[:len(s.vaultPath)-1], 0, child)
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
	node, ok := s.selectedVaultNode()
	if !ok || node.Kind != vault.EntryNode {
		return
	}
	if s.copying {
		return
	}
	s.copying = true
	s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error { return s.vault.CopyPassword(ctx, node.Path) }, func(err error) {
		s.copying = false
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
