package vault

import (
	"lazypass/internal/tui/shared"
	"lazypass/internal/vault"

	"github.com/gdamore/tcell/v2"
)

func (s *View) showVaultParentRow() bool { return len(s.vaultPath) > 0 && !s.vaultFiltering }
func (s *View) vaultParentNode() vault.Node {
	parent := s.parentPath()
	return vault.Node{Name: "..", Path: parent, Kind: vault.FolderNode}
}
func (s *View) isVaultParentNode(node vault.Node) bool {
	return node.Name == ".." && node.Kind == vault.FolderNode
}
func (s *View) vaultDisplayNodes() []vault.Node {
	nodes := s.filteredVaultNodes()
	if !s.showVaultParentRow() {
		return nodes
	}
	display := make([]vault.Node, 0, len(nodes)+1)
	display = append(display, s.vaultParentNode())
	return append(display, nodes...)
}
func nodeLabel(node vault.Node, nerdFont bool) string {
	name := node.Name
	if node.Kind == vault.FolderNode {
		if nerdFont {
			return " " + name + "/"
		}
		return name + "/"
	}
	if !node.TypeKnown {
		if nerdFont {
			return "󰦝 " + name
		}
		return name
	}
	if !nerdFont {
		return "[" + node.EntryType.String() + "] " + name
	}
	icon := ""
	switch node.EntryType {
	case vault.CardEntry:
		icon = "󰿯"
	case vault.IdentityEntry:
		icon = ""
	case vault.NoteEntry:
		icon = "󰎞"
	case vault.SSHKeyEntry:
		icon = "󰌆"
	}
	return icon + " " + name
}
func nodeColor(node vault.Node, colors shared.Palette) tcell.Color {
	if node.Kind == vault.FolderNode {
		return colors.Accent
	}
	return colors.Text
}
