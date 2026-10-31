package vault

import (
	"context"
	"slices"
	"strings"

	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

func (s *View) filteredVaultNodes() []vault.Node {
	if !s.vaultFiltering {
		return s.vaultNodes
	}
	source := s.vaultNodes
	if s.searchLoaded {
		source = s.searchNodes
	}
	needle := strings.ToLower(s.vaultFilter)
	nodes := make([]vault.Node, 0, len(source))
	for _, node := range source {
		if strings.Contains(strings.ToLower(s.searchName(node)), needle) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}
func (s *View) handleVaultFilterInput(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		s.vaultFiltering, s.vaultFilter, s.vaultSelected = false, "", 0
		s.searchNodes = nil
		s.searchLoaded = false
	case tcell.KeyCtrlU:
		s.vaultFilter, s.vaultSelected = "", 0
	case tcell.KeyCtrlW:
		s.vaultFilter, s.vaultSelected = shared.DeletePreviousWord(s.vaultFilter), 0
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		s.vaultFilter, s.vaultSelected = shared.DeleteLastRune(s.vaultFilter), 0
	case tcell.KeyUp:
		s.moveVaultSelection(-1)
	case tcell.KeyDown:
		s.moveVaultSelection(1)
	case tcell.KeyEnter:
		s.openVaultNode()
	case tcell.KeyRune:
		if event.Rune() >= ' ' {
			s.vaultFilter += string(event.Rune())
			s.vaultSelected = 0
		}
	}
}

func (s *View) searchName(node vault.Node) string {
	prefix := s.vaultPath.String()
	if prefix != "" {
		prefix += "/"
	}
	name := strings.TrimPrefix(node.Path.String(), prefix)
	if name == "" {
		return node.Name
	}
	return name
}

// Cache metadata once per search session; typing never reruns provider calls.
func (s *View) loadSearchNodes() {
	s.searchNodes = nil
	s.searchLoaded = true
	queue := []vault.Path{append(vault.Path(nil), s.vaultPath...)}
	seen := map[string]bool{s.vaultPath.String(): true}
	partial := false
	for head := 0; head < len(queue); head++ {
		path := queue[head]
		var nodes []vault.Node
		if path.String() == s.vaultPath.String() {
			nodes = s.vaultNodes
		} else {
			var err error
			nodes, err = s.vault.List(context.Background(), path)
			if err != nil {
				partial = true
				continue
			}
		}
		for _, node := range nodes {
			// Only descend into validated direct children of this folder.
			if len(node.Path) != len(path)+1 || node.Path.Validate() != nil || !slices.Equal(node.Path[:len(path)], path) {
				continue
			}
			s.searchNodes = append(s.searchNodes, node)
			if node.Kind == vault.FolderNode && !seen[node.Path.String()] {
				seen[node.Path.String()] = true
				queue = append(queue, node.Path)
			}
		}
	}
	if partial {
		s.notify(shared.Warning, "Some folders could not be searched")
	}
}
