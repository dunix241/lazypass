package bitwarden

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/gofrs/flock"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"lazypass/internal/vault"
)

const maxCloneAttachmentBytes = 64 << 20

func (p *Provider) completeClone(signature [32]byte, account string) error {
	if attempt := p.cloneAttempts[signature]; attempt != nil {
		attempt.Complete = true
	}
	if account != "" {
		return p.saveCloneJournal(account)
	}
	return nil
}

type cloneAttempt struct {
	SourceID    string                     `json:"sourceId,omitempty"`
	Destination map[string]json.RawMessage `json:"destination,omitempty"`
	Paused      bool                       `json:"paused,omitempty"`
	Complete    bool                       `json:"complete,omitempty"`
	TargetID    string                     `json:"targetId,omitempty"`
	BeforeIDs   []string                   `json:"beforeIds,omitempty"`
}

func cloneSignature(sourceID string, raw map[string]json.RawMessage) ([32]byte, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return [32]byte{}, vault.ErrMalformed
	}
	defer clear(data)
	h := sha256.New()
	h.Write([]byte(sourceID))
	h.Write([]byte{0})
	h.Write(data)
	var signature [32]byte
	copy(signature[:], h.Sum(nil))
	return signature, nil
}

func (p *Provider) findClone(ctx context.Context, raw map[string]json.RawMessage, beforeIDs []string) (string, error) {
	if _, err := p.run(ctx, []string{"sync"}, nil); err != nil {
		return "", err
	}
	out, err := p.run(ctx, []string{"list", "items"}, nil)
	defer clear(out)
	if err != nil {
		return "", err
	}
	var candidates []map[string]json.RawMessage
	if json.Unmarshal(out, &candidates) != nil {
		return "", vault.ErrMalformed
	}
	var id string
	before := make(map[string]bool, len(beforeIDs))
	for _, id := range beforeIDs {
		before[id] = true
	}
	for _, candidate := range candidates {
		var candidateID string
		_ = json.Unmarshal(candidate["id"], &candidateID)
		if before[candidateID] {
			continue
		}
		if !cloneContentsMatch(raw, candidate) {
			continue
		}
		if candidateID == "" || id != "" {
			return "", vault.ErrConflict
		}
		id = candidateID
	}
	if id == "" {
		return "", &commandFailure{vault.ErrUnavailable, "Copy result is unresolved; duplicate creation is blocked and will be rechecked on the next paste"}
	}
	return id, nil
}

type cloneAttachment struct {
	ID   string          `json:"id"`
	Name string          `json:"fileName"`
	Size json.RawMessage `json:"size"`
}

func cloneAttachments(raw map[string]json.RawMessage) ([]cloneAttachment, error) {
	var attachments []cloneAttachment
	if value := raw["attachments"]; len(value) > 0 && json.Unmarshal(value, &attachments) != nil {
		return nil, vault.ErrMalformed
	}
	var total int64
	names := make(map[string]bool, len(attachments))
	if len(attachments) > 128 {
		return nil, vault.ErrUnsupported
	}
	for _, a := range attachments {
		if a.ID == "" || a.Name == "" || a.Name == "." || a.Name == ".." || strings.ContainsAny(a.Name, "/\\\x00\r\n") {
			return nil, vault.ErrMalformed
		}
		if names[a.Name] {
			return nil, &commandFailure{vault.ErrUnsupported, "Rename duplicate attachment filenames before copying this item"}
		}
		names[a.Name] = true
		size, err := attachmentSize(a.Size)
		if err != nil {
			return nil, err
		}
		total += size
		if total > maxCloneAttachmentBytes {
			return nil, &commandFailure{vault.ErrUnsupported, "Attachment copies are limited to 64 MiB per item"}
		}
	}
	return attachments, nil
}

func attachmentSize(raw json.RawMessage) (int64, error) {
	text := string(raw)
	if strings.HasPrefix(text, "\"") {
		if json.Unmarshal(raw, &text) != nil {
			return 0, vault.ErrMalformed
		}
	}
	size, err := strconv.ParseInt(text, 10, 64)
	if err != nil || size < 0 || size > maxCloneAttachmentBytes {
		return 0, vault.ErrUnsupported
	}
	return size, nil
}

// cloneItem retains a confirmed destination ID when attachment work fails. The
// caller must reconcile that item rather than retrying creation blindly.
func (p *Provider) cloneItem(ctx context.Context, sourceID string, raw map[string]json.RawMessage) error {
	lock := flock.New(filepath.Join(p.DataDir, "lazypass-transfers.lock"))
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return err
	}
	if !locked {
		return vault.ErrUnavailable
	}
	defer lock.Unlock()
	attachments, err := cloneAttachments(raw)
	if err != nil {
		return err
	}
	prepareClone(raw)
	signature, err := cloneSignature(sourceID, raw)
	if err != nil {
		return err
	}
	if p.cloneAttempts == nil {
		p.cloneAttempts = make(map[[32]byte]*cloneAttempt)
	}
	accountStatus, err := p.status(ctx)
	if err != nil {
		return err
	}
	account := ""
	if accountStatus.UserID == "" {
		if _, native := p.Runner.(ExecRunner); native {
			return vault.ErrMalformed
		}
	}
	if accountStatus.UserID != "" {
		account = transferAccount(p.Server, accountStatus.UserID)
		if err := p.loadCloneJournal(account); err != nil {
			return err
		}
	}
	attempt, exists := p.cloneAttempts[signature]
	if exists {
		attempt.Paused = false
	}
	if !exists {
		for _, pending := range p.cloneAttempts {
			if pending.SourceID == sourceID && !pending.Complete && reflect.DeepEqual(pending.Destination, cloneDestination(raw)) {
				return &commandFailure{vault.ErrConflict, "An unfinished copy has changed source contents; duplicate creation is blocked"}
			}
		}
	}
	if exists && attempt.Complete {
		target, err := p.transferItem(ctx, attempt.TargetID, false)
		if err != nil {
			return err
		}
		if !cloneContentsMatch(raw, target) {
			return vault.ErrConflict
		}
		return nil
	}
	if !exists {
		if len(p.cloneAttempts) >= 2048 {
			return &commandFailure{vault.ErrUnavailable, "Transfer receipt capacity reached; new copies are blocked to preserve duplicate protection"}
		}
		attempt = &cloneAttempt{SourceID: sourceID, Destination: cloneDestination(raw)}
		items, err := decode[[]struct {
			ID string `json:"id"`
		}](p.run(ctx, []string{"list", "items"}, nil))
		if err != nil {
			return err
		}
		for _, item := range items {
			attempt.BeforeIDs = append(attempt.BeforeIDs, item.ID)
		}
		p.cloneAttempts[signature] = attempt
		if account != "" {
			if err := p.saveCloneJournal(account); err != nil {
				return err
			}
		}
		output, createErr := p.submit(ctx, []string{"create", "item"}, raw)
		var created struct {
			ID string `json:"id"`
		}
		if createErr == nil && json.Unmarshal(output, &created) == nil {
			attempt.TargetID = created.ID
		}
		clear(output)
		if account != "" {
			if err := p.saveCloneJournal(account); err != nil {
				return &commandFailure{vault.ErrUnavailable, "Copy outcome could not be recorded; further creation is blocked"}
			}
		}
		if ctx.Err() != nil {
			attempt.Paused = true
			if account != "" {
				_ = p.saveCloneJournal(account)
			}
			return ctx.Err()
		}
	}
	if attempt.TargetID == "" {
		for retry := 0; retry < 3; retry++ {
			if retry > 0 {
				timer := time.NewTimer(time.Duration(retry) * time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
			}
			attempt.TargetID, err = p.findClone(ctx, raw, attempt.BeforeIDs)
			if err == nil || errors.Is(err, vault.ErrConflict) {
				break
			}
		}
		if err != nil {
			return err
		}
		if account != "" {
			if err := p.saveCloneJournal(account); err != nil {
				return err
			}
		}
	}
	if exists && len(attachments) > 0 {
		target, err := p.transferItem(ctx, attempt.TargetID, false)
		if err != nil {
			return err
		}
		if !cloneContentsMatch(raw, target) {
			return vault.ErrConflict
		}
		recovery := &attachmentRecovery{provider: p, sourceID: sourceID, targetID: attempt.TargetID}
		if err := recovery.resumeLocked(ctx); err != nil {
			if ctx.Err() != nil {
				attempt.Paused = true
				if account != "" {
					_ = p.saveCloneJournal(account)
				}
			}
			return recovery
		}
		return p.completeClone(signature, account)
	}
	for _, a := range attachments {
		if err := p.copyAttachment(ctx, sourceID, attempt.TargetID, a); err != nil {
			recovery := &attachmentRecovery{provider: p, sourceID: sourceID, targetID: attempt.TargetID}
			if ctx.Err() == nil {
				if err := recovery.resumeLocked(ctx); err == nil {
					return p.completeClone(signature, account)
				}
			}
			if ctx.Err() != nil {
				attempt.Paused = true
				if account != "" {
					_ = p.saveCloneJournal(account)
				}
			}
			return recovery
		}
	}
	return p.completeClone(signature, account)
}

type attachmentRecovery struct {
	provider           *Provider
	sourceID, targetID string
}

func (r *attachmentRecovery) Error() string { return r.UserMessage() }
func (r *attachmentRecovery) UserMessage() string {
	return "Item copy exists (ID " + r.targetID + "), but attachments remain incomplete after automatic recovery; review it before copying again"
}

func (r *attachmentRecovery) resumeLocked(ctx context.Context) error {
	p := r.provider
	defer func() { p.metadata = nil; p.browser.Store(nil) }()
	if err := p.ensureUnlocked(ctx); err != nil {
		return err
	}
	if _, err := p.run(ctx, []string{"sync"}, nil); err != nil {
		return err
	}
	source, err := p.transferItem(ctx, r.sourceID, true)
	if err != nil {
		return err
	}
	target, err := p.transferItem(ctx, r.targetID, false)
	if err != nil {
		return err
	}
	for _, raw := range []map[string]json.RawMessage{source, target} {
		if value := raw["deletedDate"]; len(value) > 0 && string(value) != "null" {
			return vault.ErrNotFound
		}
	}
	attachments, err := cloneAttachments(source)
	if err != nil {
		return err
	}
	existing, err := cloneAttachments(target)
	if err != nil {
		return err
	}
	for _, a := range attachments {
		found := false
		for _, b := range existing {
			if a.Name != b.Name {
				continue
			}
			found = true
			if err := p.verifyAttachment(ctx, r.sourceID, a, r.targetID, b); err != nil {
				return err
			}
			break
		}
		if !found {
			if err := p.copyAttachment(ctx, r.sourceID, r.targetID, a); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Provider) verifyAttachment(ctx context.Context, sourceID string, a cloneAttachment, targetID string, b cloneAttachment) error {
	dir, err := os.MkdirTemp("", "lazypass-verify-")
	if err != nil {
		return vault.ErrUnavailable
	}
	defer os.RemoveAll(dir)
	var sums [2][32]byte
	for i, entry := range []struct {
		id         string
		attachment cloneAttachment
	}{{sourceID, a}, {targetID, b}} {
		path := filepath.Join(dir, strconv.Itoa(i))
		out, err := p.run(ctx, []string{"get", "attachment", entry.attachment.ID, "--itemid", entry.id, "--output", path}, nil)
		clear(out)
		if err != nil {
			return err
		}
		sums[i], err = attachmentHash(path)
		if err != nil {
			return err
		}
	}
	if sums[0] != sums[1] {
		return &commandFailure{vault.ErrConflict, "Destination attachment differs from source; review it without overwriting"}
	}
	return nil
}

func attachmentHash(path string) ([32]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return [32]byte{}, vault.ErrUnavailable
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxCloneAttachmentBytes {
		return [32]byte{}, vault.ErrMalformed
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, maxCloneAttachmentBytes+1)); err != nil {
		return [32]byte{}, vault.ErrUnavailable
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

func (p *Provider) copyAttachment(ctx context.Context, sourceID, targetID string, a cloneAttachment) error {
	dir, err := os.MkdirTemp("", "lazypass-attachment-")
	if err != nil {
		return vault.ErrUnavailable
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, a.Name)
	out, err := p.run(ctx, []string{"get", "attachment", a.ID, "--itemid", sourceID, "--output", path}, nil)
	clear(out)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	// Bitwarden attachment metadata describes encrypted size, not plaintext size.
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxCloneAttachmentBytes {
		return vault.ErrMalformed
	}
	if err := os.Chmod(path, 0600); err != nil {
		return vault.ErrUnavailable
	}
	out, err = p.run(ctx, []string{"create", "attachment", "--file", path, "--itemid", targetID}, nil)
	clear(out)
	if err != nil {
		return err
	}
	result, err := p.transferItem(ctx, targetID, false)
	if err != nil {
		return err
	}
	var copied []cloneAttachment
	if json.Unmarshal(result["attachments"], &copied) != nil {
		return vault.ErrMalformed
	}
	for _, candidate := range copied {
		if candidate.Name != a.Name {
			continue
		}
		verifyDir, err := os.MkdirTemp(dir, "verify-")
		if err != nil {
			return vault.ErrUnavailable
		}
		verify := filepath.Join(verifyDir, "attachment")
		out, err := p.run(ctx, []string{"get", "attachment", candidate.ID, "--itemid", targetID, "--output", verify}, nil)
		clear(out)
		if err != nil {
			return err
		}
		left, err := attachmentHash(path)
		if err != nil {
			return err
		}
		right, err := attachmentHash(verify)
		if err != nil || left != right {
			return vault.ErrMalformed
		}
		return nil
	}
	return vault.ErrMalformed
}
