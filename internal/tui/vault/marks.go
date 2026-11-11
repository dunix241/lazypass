package vault

import (
	"context"
	"fmt"

	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"
)

func (s *View) ClearMarks() { s.marks = nil }

func (s *View) HasMarks() bool { return len(s.marks) > 0 }

func (s *View) CancelMarkedDelete() bool {
	if s.markedDeleteCancel == nil {
		return false
	}
	s.markedDeleteCancel()
	return true
}

func markID(node vault.Node) string {
	if node.Kind == vault.FolderNode {
		return "folder:" + node.Path.String()
	}
	if node.Shared && len(node.Path) > 2 && (node.Path[0] == "Folders" || node.Path[0] == "Organizations") {
		return "item:" + node.Path[len(node.Path)-1]
	}
	return node.Path.String()
}

func (s *View) markable(node vault.Node) bool {
	if s.isVaultParentNode(node) {
		return false
	}
	if node.Kind == vault.EntryNode {
		return true
	}
	_, ok := s.vault.Provider.(vault.FolderTransfer)
	return ok && node.Kind == vault.FolderNode && !node.Protected
}

func (s *View) toggleMark(rangeMark bool) {
	nodes := s.vaultDisplayNodes()
	node, ok := s.selectedVaultNode()
	if !ok || !s.markable(node) {
		return
	}
	if s.marks == nil {
		s.marks = make(map[string]bool)
	}
	id := markID(node)
	if !rangeMark {
		if s.marks[id] {
			delete(s.marks, id)
		} else {
			s.marks[id] = true
		}
		return
	}
	nearest, distance := -1, len(nodes)+1
	for i, candidate := range nodes {
		candidateID := markID(candidate)
		if i == s.vaultSelected || !s.markable(candidate) || candidateID == id || !s.marks[candidateID] {
			continue
		}
		d := i - s.vaultSelected
		if d < 0 {
			d = -d
		}
		if d < distance {
			nearest, distance = i, d
		}
	}
	if nearest < 0 {
		s.marks[id] = true
		return
	}
	for i := min(nearest, s.vaultSelected); i <= max(nearest, s.vaultSelected); i++ {
		if s.markable(nodes[i]) {
			s.marks[markID(nodes[i])] = true
		}
	}
}

func (s *View) openMarkedDelete() {
	var nodes []vault.Node
	seen := make(map[string]bool)
	for _, node := range s.vaultDisplayNodes() {
		id := markID(node)
		if node.Kind == vault.FolderNode && s.marks[id] {
			s.notify(shared.Warning, "Unmark folders before deleting marked entries; use d or D on a single folder")
			return
		}
		if node.Kind == vault.EntryNode && s.marks[id] && !seen[id] {
			node.Path = append(vault.Path(nil), node.Path...)
			nodes = append(nodes, node)
			seen[id] = true
		}
	}
	if len(nodes) == 0 || s.deleting {
		return
	}
	if s.vault.Provider == nil || !s.vault.Provider.Capabilities().Delete {
		s.notify(shared.Warning, "Deletion unavailable")
		return
	}
	s.deleteTarget = &nodes[0]
	s.deleteDialog = shared.NewConfirmation(shared.ConfirmationOptions{
		Title:       fmt.Sprintf("Delete %d marked entries", len(nodes)),
		Message:     "Deletes the marked entries. Shared items are deleted from every collection.",
		RequireText: fmt.Sprintf("DELETE %d", len(nodes)),
		Destructive: true,
		OnCancel:    s.cancelDelete,
		OnConfirm:   func() { s.deleteMarked(nodes) },
	})
}

func (s *View) deleteMarked(nodes []vault.Node) {
	s.cancelDelete()
	s.deleting = true
	s.ClearAuthenticators()
	provider := s.vault
	var removed []vault.Node
	s.progress("Deleting marked entries…")
	request, cancel := context.WithCancel(context.Background())
	s.markedDeleteCancel = cancel
	s.deps.RunOperation.Execute(s.deps.RunPrompted, func(ctx context.Context) error {
		// The runner may retry after requesting credentials. Resume after confirmed
		// successes rather than deleting those entries a second time.
		ctx, stop := context.WithCancel(ctx)
		defer stop()
		unregister := context.AfterFunc(request, stop)
		defer unregister()
		for _, node := range nodes[len(removed):] {
			if err := request.Err(); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := provider.Delete(ctx, node); err != nil {
				return err
			}
			removed = append(removed, node)
		}
		return nil
	}, func(err error) {
		cancel()
		s.markedDeleteCancel = nil
		s.deleting = false
		for _, node := range removed {
			delete(s.marks, markID(node))
		}
		if err != nil {
			s.Refresh()
			s.notify(shared.Error, fmt.Sprintf("Deleted %d of %d marked entries; remaining entries stay marked", len(removed), len(nodes)))
			return
		}
		s.Reconcile("Deleted marked entries")
	})
}
