package bitwarden

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"lazypass/internal/vault"
)

const maxTransferJournalBytes = 1 << 20

type transferJournal struct {
	Version  int                      `json:"version"`
	Account  string                   `json:"account"`
	Attempts map[string]*cloneAttempt `json:"attempts"`
}

func (p *Provider) journalPath() string { return filepath.Join(p.DataDir, "lazypass-transfers.json") }

func (p *Provider) loadCloneJournal(account string) error {
	info, statErr := os.Lstat(p.journalPath())
	if errors.Is(statErr, os.ErrNotExist) {
		return nil
	}
	if statErr != nil {
		return vault.ErrUnavailable
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return vault.ErrMalformed
	}
	f, err := os.Open(p.journalPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return vault.ErrUnavailable
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxTransferJournalBytes {
		return vault.ErrMalformed
	}
	var journal transferJournal
	decoder := json.NewDecoder(io.LimitReader(f, maxTransferJournalBytes+1))
	if decoder.Decode(&journal) != nil || journal.Version != 1 || len(journal.Attempts) > 2048 {
		return vault.ErrMalformed
	}
	if decoder.Decode(new(any)) != io.EOF {
		return vault.ErrMalformed
	}
	if journal.Account != account {
		return &commandFailure{vault.ErrConflict, "Pending transfers belong to another account; copy was not started"}
	}
	attempts := make(map[[32]byte]*cloneAttempt, len(journal.Attempts))
	for key, attempt := range journal.Attempts {
		data, err := hex.DecodeString(key)
		if err != nil || len(data) != 32 || attempt == nil {
			return vault.ErrMalformed
		}
		if len(attempt.BeforeIDs) > 100000 || len(attempt.Destination) > 4 {
			return vault.ErrMalformed
		}
		for name, value := range attempt.Destination {
			switch name {
			case "name", "folderId", "organizationId", "collectionIds":
			default:
				return vault.ErrMalformed
			}
			if !json.Valid(value) {
				return vault.ErrMalformed
			}
		}
		var signature [32]byte
		copy(signature[:], data)
		attempts[signature] = attempt
	}
	p.cloneAttempts = attempts
	return nil
}

func (p *Provider) saveCloneJournal(account string) error {
	journal := transferJournal{Version: 1, Account: account, Attempts: make(map[string]*cloneAttempt, len(p.cloneAttempts))}
	for signature, attempt := range p.cloneAttempts {
		journal.Attempts[hex.EncodeToString(signature[:])] = attempt
	}
	return writeTransferJournal(p.journalPath(), journal)
}

func transferAccount(server, userID string) string {
	sum := sha256.Sum256([]byte(server + "\x00" + userID))
	return hex.EncodeToString(sum[:])
}
