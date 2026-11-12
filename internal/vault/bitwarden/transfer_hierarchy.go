package bitwarden

import (
	"lazypass/internal/vault"
	"strings"
)

func (p *Provider) TransferContains(parent, node vault.Node) bool {
	s := p.browser.Load()
	if s == nil || parent.Kind != vault.FolderNode {
		return false
	}
	if len(parent.Path) == 2 && parent.Path[0] == "Folders" {
		var parentName string
		for _, f := range s.folders {
			if f.ID == parent.Path[1] {
				parentName = f.Name
			}
		}
		if parentName == "" || len(node.Path) < 2 || node.Path[0] != "Folders" {
			return false
		}
		for _, f := range s.folders {
			if f.ID == node.Path[1] {
				return (node.Kind == vault.EntryNode && f.ID == parent.Path[1]) || strings.HasPrefix(f.Name, parentName+"/")
			}
		}
	}
	if len(parent.Path) == 3 && parent.Path[0] == "Organizations" && len(node.Path) >= 3 && node.Path[0] == "Organizations" && node.Path[1] == parent.Path[1] {
		a, aok := s.collectionByPath[parent.Path.String()]
		b, bok := s.collectionByPath[node.Path[:3].String()]
		return aok && bok && ((node.Kind == vault.EntryNode && a.ID == b.ID) || strings.HasPrefix(b.Name, a.Name+"/"))
	}
	return false
}
