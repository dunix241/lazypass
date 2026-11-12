package bitwarden

import (
	"encoding/json"
	"os"
	"path/filepath"

	"lazypass/internal/vault"
)

// Sync the private temporary file before publishing a journal. A remote write
// must not begin unless its intent has been recorded successfully.
func writeTransferJournal(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil || len(data) > maxTransferJournalBytes {
		return vault.ErrUnavailable
	}
	defer clear(data)
	f, err := os.CreateTemp(filepath.Dir(path), ".lazypass-journal-")
	if err != nil {
		return vault.ErrUnavailable
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return vault.ErrUnavailable
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return vault.ErrUnavailable
	}
	return nil
}
