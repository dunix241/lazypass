// Package metadata stores syncable, non-secret entry type records.
package metadata

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"lazypass/internal/vault"
)

const Directory = ".lazypass"
const typesDirectory = ".lazypass/types"
const maxRecordBytes = 64 << 10

type record struct {
	Version int             `json:"version"`
	Path    string          `json:"path"`
	Type    vault.EntryType `json:"type"`
}

func RecordPath(path vault.Path) string {
	hash := sha256.Sum256([]byte(path.String()))
	return filepath.Join(typesDirectory, hex.EncodeToString(hash[:])+".json")
}

func open(store string, create bool) (*os.Root, error) {
	r, err := os.OpenRoot(store)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{Directory, typesDirectory} {
		if create {
			if err := r.Mkdir(name, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				r.Close()
				return nil, err
			}
		}
		info, err := r.Lstat(name)
		if err != nil {
			r.Close()
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			r.Close()
			return nil, vault.ErrInvalidPath
		}
	}
	return r, nil
}

func Write(store string, path vault.Path, kind vault.EntryType) error {
	if len(path) == 0 || path.Validate() != nil || kind > vault.SSHKeyEntry {
		return vault.ErrInvalidPath
	}
	r, err := open(store, true)
	if err != nil {
		return err
	}
	defer r.Close()
	if current, err := read(r, path); err == nil && current.Type == kind {
		return nil
	}
	data, err := json.Marshal(record{Version: 1, Path: path.String(), Type: kind})
	if err != nil {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	temp := filepath.Join(typesDirectory, ".tmp-"+hex.EncodeToString(nonce[:]))
	f, err := r.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer r.Remove(temp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return r.Rename(temp, RecordPath(path))
}

func read(r *os.Root, path vault.Path) (record, error) {
	result, err := readRecord(r, RecordPath(path))
	if err == nil && result.Path != path.String() {
		return record{}, vault.ErrMalformed
	}
	return result, err
}

func readRecord(r *os.Root, name string) (record, error) {
	var result record
	info, err := r.Lstat(name)
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxRecordBytes {
		return result, vault.ErrMalformed
	}
	f, err := r.Open(name)
	if err != nil {
		return result, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() {
		return result, vault.ErrMalformed
	}
	data, err := io.ReadAll(io.LimitReader(f, maxRecordBytes+1))
	if err != nil {
		return result, err
	}
	if len(data) > maxRecordBytes || json.Unmarshal(data, &result) != nil || result.Version != 1 || result.Type > vault.SSHKeyEntry {
		return result, vault.ErrMalformed
	}
	path, err := vault.ParsePath(result.Path)
	if err != nil || len(path) == 0 || RecordPath(path) != name {
		return record{}, vault.ErrMalformed
	}
	return result, nil
}

func Decorate(store string, nodes []vault.Node) {
	for i := range nodes {
		if nodes[i].Kind == vault.EntryNode {
			nodes[i].EntryType = vault.LoginEntry
			nodes[i].TypeKnown = false
		}
	}
	if len(nodes) == 0 {
		return
	}
	r, err := open(store, false)
	if err != nil {
		return
	}
	defer r.Close()
	for i := range nodes {
		if nodes[i].Kind != vault.EntryNode {
			continue
		}
		if value, err := read(r, nodes[i].Path); err == nil {
			nodes[i].EntryType = value.Type
			nodes[i].TypeKnown = true
		}
	}
}

func Remove(store string, path vault.Path, recursive bool) error {
	if len(path) == 0 || path.Validate() != nil {
		return vault.ErrInvalidPath
	}
	r, err := open(store, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer r.Close()
	if !recursive {
		err := r.Remove(RecordPath(path))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	f, err := r.Open(typesDirectory)
	if err != nil {
		return err
	}
	defer f.Close()
	files, err := f.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, file := range files {
		if !file.Type().IsRegular() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		name := filepath.Join(typesDirectory, file.Name())
		value, err := readRecord(r, name)
		if err != nil || !strings.HasPrefix(value.Path, path.String()+"/") {
			continue
		}
		if err := r.Remove(name); err != nil {
			return err
		}
	}
	return nil
}
