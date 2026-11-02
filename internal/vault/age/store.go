package age

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	agecrypto "filippo.io/age"
	"lazypass/internal/vault"
)

func (p *Provider) open(ctx context.Context) (*os.Root, error) {
	if ctx.Err() != nil {
		return nil, vault.ErrUnavailable
	}
	r, err := os.OpenRoot(p.StoreDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, vault.ErrUninitialized
	}
	if err != nil {
		return nil, vault.ErrUnavailable
	}
	return r, nil
}
func checkedPath(path vault.Path, entry bool) (string, error) {
	if err := path.Validate(); err != nil {
		return "", err
	}
	if entry && len(path) == 0 {
		return "", vault.ErrInvalidPath
	}
	for _, part := range path {
		if part == manifestName || strings.HasPrefix(part, ".lazypass-") || strings.HasSuffix(part, ".age") {
			return "", vault.ErrInvalidPath
		}
	}
	if len(path) == 0 {
		return ".", nil
	}
	return filepath.Join([]string(path)...), nil
}
func checkParents(root *os.Root, path vault.Path, create bool) error {
	for i := range path {
		name := filepath.Join([]string(path[:i+1])...)
		if create {
			if err := root.Mkdir(name, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return vault.ErrUnavailable
			}
		}
		info, err := root.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			return vault.ErrNotFound
		}
		if err != nil {
			return vault.ErrUnavailable
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return vault.ErrInvalidPath
		}
		if !info.IsDir() {
			return vault.ErrConflict
		}
	}
	return nil
}
func readRegular(root *os.Root, name string) ([]byte, error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil, vault.ErrNotFound
	}
	if err != nil {
		return nil, vault.ErrUnavailable
	}
	if !info.Mode().IsRegular() {
		return nil, vault.ErrInvalidPath
	}
	if info.Size() > maxFileBytes {
		return nil, vault.ErrMalformed
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, vault.ErrUnavailable
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() {
		return nil, vault.ErrInvalidPath
	}
	data, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		return nil, vault.ErrUnavailable
	}
	if len(data) > maxFileBytes {
		return nil, vault.ErrMalformed
	}
	return data, nil
}
func (p *Provider) List(ctx context.Context, path vault.Path) ([]vault.Node, error) {
	name, err := checkedPath(path, false)
	if err != nil {
		return nil, err
	}
	r, err := p.open(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	if _, err := readManifest(r); err != nil {
		return nil, err
	}
	if err := checkParents(r, path, false); err != nil {
		return nil, err
	}
	f, err := r.Open(name)
	if err != nil {
		return nil, vault.ErrUnavailable
	}
	defer f.Close()
	items, err := f.ReadDir(-1)
	if err != nil {
		return nil, vault.ErrUnavailable
	}
	var nodes []vault.Node
	for _, item := range items {
		if ctx.Err() != nil {
			return nil, vault.ErrUnavailable
		}
		name := item.Name()
		if name == manifestName || strings.HasPrefix(name, ".lazypass-") || item.Type()&os.ModeSymlink != 0 {
			continue
		}
		kind := vault.FolderNode
		if !item.IsDir() {
			if !item.Type().IsRegular() || !strings.HasSuffix(name, ".age") {
				continue
			}
			name = strings.TrimSuffix(name, ".age")
			kind = vault.EntryNode
		}
		child := append(append(vault.Path(nil), path...), name)
		if _, err := checkedPath(child, true); err != nil {
			continue
		}
		nodes = append(nodes, vault.Node{Path: child, Name: name, Kind: kind})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Kind != nodes[j].Kind {
			return nodes[i].Kind == vault.FolderNode
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}
func (p *Provider) Read(ctx context.Context, path vault.Path) (vault.Entry, error) {
	name, err := checkedPath(path, true)
	if err != nil {
		return vault.Entry{}, err
	}
	r, err := p.open(ctx)
	if err != nil {
		return vault.Entry{}, err
	}
	defer r.Close()
	if _, err := readManifest(r); err != nil {
		return vault.Entry{}, err
	}
	if err := checkParents(r, path[:len(path)-1], false); err != nil {
		return vault.Entry{}, err
	}
	data, err := readRegular(r, name+".age")
	if err != nil {
		return vault.Entry{}, err
	}
	ids, err := p.unlock(ctx)
	if err != nil {
		return vault.Entry{}, err
	}
	reader, err := agecrypto.Decrypt(bytes.NewReader(data), ids...)
	if err != nil {
		return vault.Entry{}, vault.ErrLocked
	}
	plain, err := io.ReadAll(io.LimitReader(reader, maxFileBytes+1))
	if err != nil {
		return vault.Entry{}, vault.ErrMalformed
	}
	defer clear(plain)
	if len(plain) > maxFileBytes {
		return vault.Entry{}, vault.ErrMalformed
	}
	return vault.DecodeEntry(path, string(plain))
}
func (p *Provider) Write(ctx context.Context, entry vault.Entry) error {
	return p.write(ctx, entry, false)
}
func (p *Provider) Create(ctx context.Context, entry vault.Entry) error {
	return p.write(ctx, entry, true)
}
func (p *Provider) write(ctx context.Context, entry vault.Entry, create bool) error {
	name, err := checkedPath(entry.Path, true)
	if err != nil {
		return err
	}
	plain, err := vault.EncodeEntry(entry)
	if err != nil {
		return err
	}
	if len(plain) > maxFileBytes-1024 {
		return vault.ErrMalformed
	}
	r, err := p.open(ctx)
	if err != nil {
		return err
	}
	defer r.Close()
	m, err := readManifest(r)
	if err != nil {
		return err
	}
	if err := checkParents(r, entry.Path[:len(entry.Path)-1], true); err != nil {
		return err
	}
	var recipients []agecrypto.Recipient
	for _, value := range m.Recipients {
		recipient, _ := agecrypto.ParseX25519Recipient(value)
		recipients = append(recipients, recipient)
	}
	var encrypted bytes.Buffer
	w, err := agecrypto.Encrypt(&encrypted, recipients...)
	if err != nil {
		return vault.ErrUnavailable
	}
	if _, err := io.WriteString(w, plain); err != nil {
		return vault.ErrUnavailable
	}
	if w.Close() != nil {
		return vault.ErrUnavailable
	}
	if encrypted.Len() > maxFileBytes {
		return vault.ErrMalformed
	}
	if ctx.Err() != nil {
		return vault.ErrUnavailable
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return vault.ErrUnavailable
	}
	temp := filepath.Join(filepath.Dir(name), ".lazypass-"+hex.EncodeToString(random[:]))
	f, err := r.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return vault.ErrUnavailable
	}
	defer r.Remove(temp)
	if _, err := f.Write(encrypted.Bytes()); err != nil {
		f.Close()
		return vault.ErrUnavailable
	}
	if f.Close() != nil {
		return vault.ErrUnavailable
	}
	if ctx.Err() != nil {
		return vault.ErrUnavailable
	}
	if create {
		err = r.Link(temp, name+".age")
	} else {
		err = r.Rename(temp, name+".age")
	}
	if errors.Is(err, os.ErrExist) {
		return vault.ErrConflict
	}
	if err != nil {
		return vault.ErrUnavailable
	}
	return nil
}
func (p *Provider) CreateFolder(ctx context.Context, path vault.Path) error {
	if _, err := checkedPath(path, true); err != nil {
		return err
	}
	r, err := p.open(ctx)
	if err != nil {
		return err
	}
	defer r.Close()
	if _, err := readManifest(r); err != nil {
		return err
	}
	return checkParents(r, path, true)
}
func (p *Provider) Delete(ctx context.Context, path vault.Path) error {
	return p.remove(ctx, path, false)
}
func (p *Provider) DeleteFolder(ctx context.Context, path vault.Path) error {
	return p.remove(ctx, path, true)
}
func (p *Provider) remove(ctx context.Context, path vault.Path, folder bool) error {
	name, err := checkedPath(path, true)
	if err != nil {
		return err
	}
	r, err := p.open(ctx)
	if err != nil {
		return err
	}
	defer r.Close()
	if _, err := readManifest(r); err != nil {
		return err
	}
	parents := path[:len(path)-1]
	if folder {
		parents = path
	}
	if err := checkParents(r, parents, false); err != nil {
		return err
	}
	if folder {
		err = r.RemoveAll(name)
	} else {
		info, err := r.Lstat(name + ".age")
		if errors.Is(err, os.ErrNotExist) {
			return vault.ErrNotFound
		}
		if err != nil {
			return vault.ErrUnavailable
		}
		if !info.Mode().IsRegular() {
			return vault.ErrInvalidPath
		}
		err = r.Remove(name + ".age")
	}
	if err != nil {
		return vault.ErrUnavailable
	}
	return nil
}
