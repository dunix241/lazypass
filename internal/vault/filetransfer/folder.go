package filetransfer

import (
	"context"
	"errors"
	"lazypass/internal/vault"
	"os"
	"path/filepath"
	"strings"
)

type Tree struct{ Files, Directories []string }

// Inspect rejects links and unknown files rather than silently losing contents.
func Inspect(ctx context.Context, root *os.Root, source, target, extension string) (Tree, error) {
	var tree Tree
	if source == target {
		return tree, vault.ErrConflict
	}
	if strings.HasPrefix(target, source+string(filepath.Separator)) {
		return tree, vault.ErrInvalidPath
	}
	if _, err := root.Lstat(target + extension); err == nil {
		return tree, vault.ErrConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return tree, vault.ErrUnavailable
	}
	if _, err := root.Lstat(target); err == nil {
		return tree, vault.ErrConflict
	} else if !errors.Is(err, os.ErrNotExist) {
		return tree, vault.ErrUnavailable
	}
	var walk func(string) error
	walk = func(relative string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		path := filepath.Join(source, relative)
		info, err := root.Lstat(path)
		if err != nil {
			return vault.ErrNotFound
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return vault.ErrInvalidPath
		}
		tree.Directories = append(tree.Directories, relative)
		f, err := root.Open(path)
		if err != nil {
			return vault.ErrUnavailable
		}
		entries, err := f.ReadDir(-1)
		f.Close()
		if err != nil {
			return vault.ErrUnavailable
		}
		for _, entry := range entries {
			child := filepath.Join(relative, entry.Name())
			if entry.IsDir() {
				if err := walk(child); err != nil {
					return err
				}
				continue
			}
			if !entry.Type().IsRegular() {
				return vault.ErrInvalidPath
			}
			supported := strings.HasSuffix(entry.Name(), extension) || (extension == ".gpg" && (entry.Name() == ".gpg-id" || entry.Name() == ".gpg-id.sig"))
			if !supported {
				return vault.ErrUnsupported
			}
			tree.Files = append(tree.Files, child)
		}
		return nil
	}
	return tree, walk("")
}

// ExecuteTree reserves the destination exclusively and copies every file before
// removing any source. A failure during removal leaves both trees for review.
func ExecuteTree(ctx context.Context, root *os.Root, source, target string, tree Tree, move bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := root.Mkdir(target, 0700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return vault.ErrConflict
		}
		return vault.ErrUnavailable
	}
	published := false
	defer func() {
		if !published {
			_ = root.RemoveAll(target)
		}
	}()
	for _, dir := range tree.Directories {
		if dir != "" {
			if err := root.Mkdir(filepath.Join(target, dir), 0700); err != nil {
				return vault.ErrUnavailable
			}
		}
	}
	originals := make(map[string]os.FileInfo, len(tree.Files))
	for _, file := range tree.Files {
		info, err := root.Lstat(filepath.Join(source, file))
		if err != nil || !info.Mode().IsRegular() {
			return vault.ErrConflict
		}
		originals[file] = info
		if err := Execute(ctx, root, filepath.Join(source, file), filepath.Join(target, file), false); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	published = true
	if move {
		// Check the whole source set before deleting anything. This catches a
		// replaced or edited file during copying; external writers are not locked.
		for _, file := range tree.Files {
			info, err := root.Lstat(filepath.Join(source, file))
			original := originals[file]
			if err != nil || !os.SameFile(original, info) || original.Size() != info.Size() || !original.ModTime().Equal(info.ModTime()) {
				return errors.New("folder copied, but source changed; source retained for review")
			}
		}
		for _, file := range tree.Files {
			if err := root.Remove(filepath.Join(source, file)); err != nil {
				return errors.New("folder copied, but source cleanup failed; review both locations")
			}
		}
		for i := len(tree.Directories) - 1; i >= 0; i-- {
			if err := root.Remove(filepath.Join(source, tree.Directories[i])); err != nil {
				return errors.New("folder copied, but source cleanup failed; review both locations")
			}
		}
	}
	return nil
}
