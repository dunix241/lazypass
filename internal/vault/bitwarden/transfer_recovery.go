package bitwarden

import (
	"context"
	"errors"
	"github.com/gofrs/flock"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lazypass/internal/vault"
)

// RecoverTransfers is called by the application scheduler. It only uses cached
// credentials and processes a bounded number of jobs per invocation.
func (p *Provider) RecoverTransfers(ctx context.Context) (vault.TransferRecoveryResult, error) {
	var result vault.TransferRecoveryResult
	if !p.mu.TryLock() {
		return result, nil
	}
	defer p.mu.Unlock()
	if p.session == "" {
		return result, nil
	}
	if err := p.recoverStandaloneClones(ctx, &result); err != nil {
		return result, err
	}
	entries, err := os.ReadDir(p.DataDir)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, vault.ErrUnavailable
	}
	var paths []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "lazypass-hierarchy-") && strings.HasSuffix(entry.Name(), ".json") && entry.Type().IsRegular() {
			paths = append(paths, filepath.Join(p.DataDir, entry.Name()))
		}
	}
	if len(paths) == 0 {
		return result, nil
	}
	unlock, err := p.hierarchyLock(ctx)
	if err != nil {
		return result, err
	}
	defer unlock()
	processed := 0
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		job, err := p.loadHierarchy(ctx, path)
		if err != nil {
			result.Failed++
			continue
		}
		if job == nil {
			continue
		}
		if job.Complete {
			continue
		}
		if job.Failure != "" {
			result.Failed++
			continue
		}
		if time.Now().Before(job.RetryAfter) {
			result.Pending++
			continue
		}
		if job.Paused {
			result.Pending++
			continue
		}
		if processed >= 2 {
			result.Pending++
			continue
		}
		processed++
		if err := p.runHierarchyCopy(ctx, path, job); err != nil {
			if errors.Is(err, vault.ErrConflict) || errors.Is(err, vault.ErrMalformed) || errors.Is(err, vault.ErrUnsupported) {
				result.Failed++
			} else {
				result.Pending++
			}
			continue
		}
		result.Completed++
	}
	if result.Completed > 0 {
		p.metadata = nil
		p.browser.Store(nil)
	}
	return result, nil
}

func (p *Provider) recoverStandaloneClones(ctx context.Context, result *vault.TransferRecoveryResult) error {
	lock := flock.New(filepath.Join(p.DataDir, "lazypass-transfers.lock"))
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer lock.Unlock()
	status, err := p.status(ctx)
	if err != nil {
		return err
	}
	if status.UserID == "" {
		return vault.ErrMalformed
	}
	account := transferAccount(p.Server, status.UserID)
	if p.cloneAttempts == nil {
		p.cloneAttempts = make(map[[32]byte]*cloneAttempt)
	}
	if err := p.loadCloneJournal(account); err != nil {
		return err
	}
	processed := 0
	for signature, attempt := range p.cloneAttempts {
		if attempt.Complete {
			continue
		}
		if attempt.Paused || attempt.SourceID == "" {
			result.Pending++
			continue
		}
		if processed >= 2 {
			result.Pending++
			continue
		}
		processed++
		if err := ctx.Err(); err != nil {
			return err
		}
		raw, err := p.transferItem(ctx, attempt.SourceID, true)
		if err != nil {
			result.Pending++
			continue
		}
		for key, value := range attempt.Destination {
			raw[key] = value
		}
		prepareClone(raw)
		current, err := cloneSignature(attempt.SourceID, raw)
		if err != nil || current != signature {
			result.Failed++
			continue
		}
		if attempt.TargetID == "" {
			attempt.TargetID, err = p.findClone(ctx, raw, attempt.BeforeIDs)
			if err != nil {
				result.Pending++
				continue
			}
			if err := p.saveCloneJournal(account); err != nil {
				return err
			}
		}
		recovery := attachmentRecovery{provider: p, sourceID: attempt.SourceID, targetID: attempt.TargetID}
		target, err := p.transferItem(ctx, attempt.TargetID, false)
		if err != nil {
			result.Pending++
			continue
		}
		if !cloneContentsMatch(raw, target) {
			result.Failed++
			continue
		}
		if err := recovery.resumeLocked(ctx); err != nil {
			result.Pending++
			continue
		}
		// Keep a completed receipt until any parent hierarchy has recorded it.
		attempt.Complete = true
		if err := p.saveCloneJournal(account); err != nil {
			return err
		}
		result.Completed++
	}
	return nil
}
