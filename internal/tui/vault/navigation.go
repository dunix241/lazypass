package vault

import (
	"context"
	"errors"

	"lazypass/internal/debug"
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
)

func (s *View) Refresh() {
	s.vaultError = ""
	nodes, err := s.vault.List(context.Background(), s.vaultPath)
	if err != nil {
		if s.vault.Provider == nil {
			return
		}
		debug.Failure("vault list", vaultFailureCause(err))
		s.vaultError = "Vault unavailable or folder cannot be opened."
		nodes = nil
	}
	s.vaultNodes = nodes
	if s.vaultFiltering {
		s.loadSearchNodes()
	} else {
		s.searchNodes = nil
		s.searchLoaded = false
	}
	s.vaultSelected = min(s.vaultSelected, max(0, len(s.vaultDisplayNodes())-1))
}
func (s *View) moveVaultSelection(delta int) {
	nodes := s.vaultDisplayNodes()
	s.vaultSelected = min(max(0, len(nodes)-1), max(0, s.vaultSelected+delta))
}
func (s *View) vaultParent() {
	if len(s.vaultPath) == 0 {
		return
	}
	child := s.vaultPath
	s.vaultFiltering, s.vaultFilter = false, ""
	s.vaultPath = s.vaultPath[:len(s.vaultPath)-1]
	s.Refresh()
	for i, node := range s.vaultDisplayNodes() {
		if !s.isVaultParentNode(node) && node.Path.String() == child.String() {
			s.vaultSelected = i
			return
		}
	}
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
		s.vaultPath, s.vaultSelected, s.vaultFiltering, s.vaultFilter = node.Path, 1, false, ""
		s.Refresh()
		return
	}
	s.copyVaultEntry()
}
func (s *View) copyVaultEntry() {
	node, ok := s.selectedVaultNode()
	if !ok || node.Kind != vault.EntryNode {
		return
	}
	if err := s.deps.RunPrompted.Execute(func(ctx context.Context) error { return s.vault.CopyPassword(ctx, node.Path) }); err != nil {
		var diagnostic vault.Diagnostic
		if !errors.As(err, &diagnostic) {
			debug.Failure("vault copy", vaultFailureCause(err))
		}
		s.notify(shared.Error, copyErrorMessage(err))
		return
	}
	s.notify(shared.Success, "Copied to clipboard")
}
func (s *View) selectedVaultNode() (vault.Node, bool) {
	nodes := s.vaultDisplayNodes()
	if s.vaultSelected < 0 || s.vaultSelected >= len(nodes) {
		return vault.Node{}, false
	}
	return nodes[s.vaultSelected], true
}
