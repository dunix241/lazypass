package config

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"

	"lazypass/internal/build"
	"lazypass/internal/update"
)

const schemaComment = "# yaml-language-server:"

func schemaURL(version string) string {
	if update.Stable(version) {
		return "https://github.com/dunix241/lazypass/releases/download/v" + strings.TrimPrefix(version, "v") + "/config.schema.json"
	}
	return "https://raw.githubusercontent.com/dunix241/lazypass/main/schemas/config.schema.json"
}

func schemaDirective() string {
	return schemaComment + " $schema=" + schemaURL(build.Version) + "\n\n"
}

// SyncSchema updates only the editor directive of an existing config. Missing
// files are left to the normal first-save workflow.
func SyncSchema(ctx context.Context, path string) error {
	if path == "" {
		return nil
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("config is not a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := withSchemaDirective(data)
	if bytes.Equal(data, updated) {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return replaceUnchanged(path, data, updated)
}

func withSchemaDirective(data []byte) []byte {
	newline := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		newline = "\r\n"
	}
	directive := schemaComment + " $schema=" + schemaURL(build.Version)
	var result bytes.Buffer
	result.Grow(len(data) + len(directive))
	found := false
	for remaining := data; len(remaining) > 0; {
		end := bytes.IndexByte(remaining, '\n') + 1
		if end == 0 {
			end = len(remaining)
		}
		line := remaining[:end]
		remaining = remaining[end:]
		if bytes.HasPrefix(line, []byte(schemaComment)) && bytes.Contains(line, []byte("$schema=")) {
			if !found {
				result.WriteString(directive)
				if bytes.HasSuffix(line, []byte("\n")) {
					result.WriteString(newline)
				}
				found = true
			}
			continue
		}
		result.Write(line)
	}
	if found {
		return result.Bytes()
	}
	return append([]byte(directive+newline+newline), data...)
}
