// Package browsercache persists browser metadata, never decrypted entry contents.
package browsercache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"lazypass/internal/util"
	"lazypass/internal/vault"
)

const maxBytes = 8 << 20

type Store struct {
	Path string
	mu   sync.Mutex
}
type document struct {
	Version   int                     `json:"version"`
	Locations map[string][]vault.Node `json:"locations"`
}

func New(identity string) *Store {
	root, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	digest := sha256.Sum256([]byte(identity))
	return &Store{Path: filepath.Join(root, "lazypass", "browser", hex.EncodeToString(digest[:])+".json")}
}

func (s *Store) Scoped(identity string) *Store {
	digest := sha256.Sum256([]byte(s.Path + "\x00" + identity))
	return &Store{Path: filepath.Join(filepath.Dir(s.Path), hex.EncodeToString(digest[:])+".json")}
}

func (s *Store) load() (document, error) {
	doc := document{Version: 1, Locations: make(map[string][]vault.Node)}
	f, err := os.Open(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return doc, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return doc, err
	}
	if len(data) > maxBytes {
		return doc, errors.New("browser cache exceeds limit")
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, err
	}
	if doc.Version != 1 || doc.Locations == nil {
		return doc, errors.New("invalid browser cache")
	}
	for key, nodes := range doc.Locations {
		parent, err := vault.ParsePath(key)
		if err != nil {
			return doc, err
		}
		for _, node := range nodes {
			if node.Path.Validate() != nil || len(node.Path) == 0 || node.Path.String() == parent.String() || node.Kind > vault.EntryNode {
				return doc, errors.New("invalid cached node")
			}
		}
	}
	return doc, nil
}

func (s *Store) Load() (map[string][]vault.Node, error) {
	if s == nil {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.load()
	if err != nil {
		return nil, err
	}
	return doc.Locations, nil
}

func (s *Store) Save(path vault.Path, nodes []vault.Node) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	doc, err := s.load()
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return err
		}
		doc = document{Version: 1, Locations: make(map[string][]vault.Node)}
	}
	if existing, ok := doc.Locations[path.String()]; ok && reflect.DeepEqual(existing, nodes) {
		return nil
	}
	doc.Locations[path.String()] = nodes
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	if len(data) > maxBytes {
		return errors.New("browser cache exceeds limit")
	}
	return util.WritePrivateFile(s.Path, data)
}
