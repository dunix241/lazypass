// Package filetransfer publishes native encrypted entries without overwriting.
package filetransfer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"lazypass/internal/vault"
	"os"
	"path/filepath"
)

func Check(root *os.Root, source, target string) error {
	info, err := root.Lstat(source)
	if errors.Is(err, os.ErrNotExist) {
		return vault.ErrNotFound
	}
	if err != nil {
		return vault.ErrUnavailable
	}
	if !info.Mode().IsRegular() {
		return vault.ErrInvalidPath
	}
	if _, err := root.Lstat(target); err == nil {
		return vault.ErrConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return vault.ErrUnavailable
	}
	return nil
}

func Execute(ctx context.Context, root *os.Root, source, target string, move bool) error {
	if err := Check(root, source, target); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if move {
		if err := root.Link(source, target); err != nil {
			if errors.Is(err, os.ErrExist) {
				return vault.ErrConflict
			}
			return vault.ErrUnavailable
		}
		if err := root.Remove(source); err != nil {
			return errors.New("destination saved, but source removal failed; both entries remain")
		}
		return nil
	}
	in, err := root.Open(source)
	if err != nil {
		return vault.ErrUnavailable
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return vault.ErrInvalidPath
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return vault.ErrUnavailable
	}
	temp := filepath.Join(filepath.Dir(target), ".lazypass-create-"+hex.EncodeToString(nonce[:]))
	out, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return vault.ErrConflict
	}
	if err != nil {
		return vault.ErrUnavailable
	}
	defer root.Remove(temp)
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		return vault.ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := root.Link(temp, target); err != nil {
		if errors.Is(err, os.ErrExist) {
			return vault.ErrConflict
		}
		return vault.ErrUnavailable
	}
	return nil
}
