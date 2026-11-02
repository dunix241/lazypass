package service

import (
	"slices"
	"strings"
	"sync"

	"lazypass/internal/vault"
)

// TypeCache remembers decrypted type metadata, never entry values.
type TypeCache struct {
	mu    sync.RWMutex
	types map[string]vault.EntryType
}

func (c *TypeCache) forget(node vault.Node) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	path := node.Path.String()
	if node.Kind == vault.EntryNode {
		delete(c.types, path)
		return
	}
	for key := range c.types {
		if strings.HasPrefix(key, path+"/") {
			delete(c.types, key)
		}
	}
}

func (c *TypeCache) remember(entry vault.Entry) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.types == nil {
		c.types = make(map[string]vault.EntryType)
	}
	c.types[entry.Path.String()] = entry.Type
}

func (c *TypeCache) decorate(nodes []vault.Node) []vault.Node {
	if c == nil {
		return nodes
	}
	nodes = slices.Clone(nodes)
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := range nodes {
		if nodes[i].Kind != vault.EntryNode || nodes[i].TypeKnown {
			continue
		}
		if kind, ok := c.types[nodes[i].Path.String()]; ok {
			nodes[i].EntryType = kind
			nodes[i].TypeKnown = true
		}
	}
	return nodes
}

func (c *TypeCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.types = nil
}
