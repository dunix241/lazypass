package bitwarden

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"lazypass/internal/vault"
)

type hierarchyContainer struct {
	OriginalName   string `json:"originalName,omitempty"`
	SourceID       string `json:"sourceId"`
	OrganizationID string `json:"organizationId,omitempty"`
	Name           string `json:"name"`
	TargetID       string `json:"targetId,omitempty"`
	Attempted      bool   `json:"attempted,omitempty"`
}

type hierarchyItem struct {
	Fingerprint string   `json:"fingerprint,omitempty"`
	SourceID    string   `json:"sourceId"`
	Containers  []string `json:"containers"`
	Complete    bool     `json:"complete,omitempty"`
}

type hierarchyJob struct {
	Failure    string               `json:"failure,omitempty"`
	Retries    int                  `json:"retries,omitempty"`
	RetryAfter time.Time            `json:"retryAfter,omitempty"`
	Complete   bool                 `json:"complete,omitempty"`
	Move       bool                 `json:"move,omitempty"`
	Paused     bool                 `json:"paused,omitempty"`
	Version    int                  `json:"version"`
	Account    string               `json:"account"`
	Containers []hierarchyContainer `json:"containers"`
	Items      []hierarchyItem      `json:"items"`
}

func (p *Provider) hierarchyPath(source, destination vault.Path, name string) string {
	data, _ := json.Marshal([]any{p.Server, source, destination, name})
	sum := sha256.Sum256(data)
	return filepath.Join(p.DataDir, "lazypass-hierarchy-"+hex.EncodeToString(sum[:])+".json")
}

func (p *Provider) hierarchyOperationPath(source, destination vault.Path, name string, move bool) string {
	path := p.hierarchyPath(source, destination, name)
	if move {
		return path + ".move.json"
	}
	return path
}

func (p *Provider) loadHierarchy(ctx context.Context, path string) (*hierarchyJob, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, vault.ErrUnavailable
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > maxTransferJournalBytes {
		return nil, vault.ErrMalformed
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, vault.ErrUnavailable
	}
	defer f.Close()
	var job hierarchyJob
	decoder := json.NewDecoder(io.LimitReader(f, maxTransferJournalBytes+1))
	if decoder.Decode(&job) != nil || job.Version != 1 || len(job.Containers) == 0 || len(job.Containers) > 4096 || len(job.Items) > 100000 {
		return nil, vault.ErrMalformed
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, vault.ErrMalformed
	}
	ids := make(map[string]bool, len(job.Containers))
	for _, c := range job.Containers {
		if c.SourceID == "" || c.Name == "" || ids[c.SourceID] {
			return nil, vault.ErrMalformed
		}
		ids[c.SourceID] = true
	}
	for _, item := range job.Items {
		if item.SourceID == "" || len(item.Containers) == 0 {
			return nil, vault.ErrMalformed
		}
		for _, id := range item.Containers {
			if !ids[id] {
				return nil, vault.ErrMalformed
			}
		}
	}
	account, err := p.status(ctx)
	if err != nil {
		return nil, err
	}
	if account.UserID == "" || job.Account != transferAccount(p.Server, account.UserID) {
		return nil, vault.ErrConflict
	}
	return &job, nil
}

func saveHierarchy(path string, job *hierarchyJob) error {
	return writeTransferJournal(path, job)
}

func (p *Provider) newHierarchy(ctx context.Context, s snapshot, plan []containerTransfer) (*hierarchyJob, error) {
	account, err := p.status(ctx)
	if err != nil {
		return nil, err
	}
	if account.UserID == "" {
		return nil, vault.ErrMalformed
	}
	job := &hierarchyJob{Version: 1, Account: transferAccount(p.Server, account.UserID)}
	selected := map[string]bool{}
	for _, c := range plan {
		job.Containers = append(job.Containers, hierarchyContainer{SourceID: c.id, OriginalName: c.source, OrganizationID: c.organizationID, Name: c.target})
		selected[c.id] = true
	}
	organization := plan[0].organizationID != ""
	seen := map[string]bool{}
	for _, entry := range append(append([]item(nil), s.items...), s.excluded...) {
		if seen[entry.ID] {
			continue
		}
		var containers []string
		if organization {
			for _, id := range entry.CollectionIDs {
				if selected[id] {
					containers = append(containers, id)
				}
			}
		} else if selected[entry.FolderID] {
			containers = []string{entry.FolderID}
		}
		if len(containers) > 0 {
			raw, err := p.transferItem(ctx, entry.ID, true)
			if err != nil {
				return nil, err
			}
			fingerprint, err := cloneSignature(entry.ID, raw)
			if err != nil {
				return nil, err
			}
			job.Items = append(job.Items, hierarchyItem{SourceID: entry.ID, Containers: containers, Fingerprint: hex.EncodeToString(fingerprint[:])})
			seen[entry.ID] = true
		}
	}
	return job, nil
}

func (p *Provider) newMoveHierarchy(ctx context.Context, plan []containerTransfer) (*hierarchyJob, error) {
	account, err := p.status(ctx)
	if err != nil {
		return nil, err
	}
	if account.UserID == "" {
		return nil, vault.ErrMalformed
	}
	job := &hierarchyJob{Version: 1, Account: transferAccount(p.Server, account.UserID), Move: true}
	for _, c := range plan {
		job.Containers = append(job.Containers, hierarchyContainer{SourceID: c.id, OriginalName: c.source, OrganizationID: c.organizationID, Name: c.target})
	}
	return job, nil
}

func (p *Provider) runHierarchyCopy(ctx context.Context, path string, job *hierarchyJob) (result error) {
	if job.Complete {
		return nil
	}
	job.Paused = false
	defer func() {
		if ctx.Err() != nil {
			job.Paused = true
			if err := saveHierarchy(path, job); err != nil {
				result = err
			}
		}
		if result != nil && ctx.Err() == nil {
			job.Retries++
			delay := time.Duration(1<<min(job.Retries, 7)) * 15 * time.Second
			job.RetryAfter = time.Now().Add(delay)
			if errors.Is(result, vault.ErrConflict) || errors.Is(result, vault.ErrMalformed) || errors.Is(result, vault.ErrUnsupported) {
				job.Failure = "Transfer blocked by changed data, permissions, or invalid state"
			}
			if err := saveHierarchy(path, job); err != nil {
				result = err
			}
		}
	}()
	if job.Move {
		return p.runHierarchyMove(ctx, path, job)
	}
	for i := range job.Containers {
		c := &job.Containers[i]
		if err := ctx.Err(); err != nil {
			return err
		}
		if c.TargetID != "" {
			if err := p.verifyContainer(ctx, *c); err != nil {
				return err
			}
			continue
		}
		if c.Attempted {
			id, err := p.reconcileContainer(ctx, *c)
			if err != nil {
				return err
			}
			c.TargetID = id
		} else {
			args := []string{"create", "folder"}
			payload := map[string]any{"name": c.Name}
			if c.OrganizationID != "" {
				payload = collectionClonePayload(containerTransfer{organizationID: c.OrganizationID, target: c.Name})
				args = []string{"create", "org-collection", "--organizationid", c.OrganizationID}
			}
			if err := p.ensureContainerAbsent(ctx, *c); err != nil {
				return err
			}
			c.Attempted = true
			if err := saveHierarchy(path, job); err != nil {
				return err
			}
			out, err := p.submit(ctx, args, payload)
			var created struct {
				ID string `json:"id"`
			}
			if err == nil && json.Unmarshal(out, &created) == nil {
				c.TargetID = created.ID
			}
			clear(out)
			if c.TargetID == "" {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				c.TargetID, err = p.reconcileContainer(ctx, *c)
				if err != nil {
					return err
				}
			}
		}
		if err := saveHierarchy(path, job); err != nil {
			return err
		}
		if c.OrganizationID != "" {
			if err := p.verifyContainer(ctx, *c); err != nil {
				return err
			}
		}
	}
	mapping := map[string]string{}
	for _, c := range job.Containers {
		mapping[c.SourceID] = c.TargetID
	}
	for i := range job.Items {
		entry := &job.Items[i]
		if entry.Complete {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		raw, err := p.transferItem(ctx, entry.SourceID, true)
		if err != nil {
			return err
		}
		fingerprint, err := cloneSignature(entry.SourceID, raw)
		if err != nil {
			return err
		}
		if entry.Fingerprint != "" && entry.Fingerprint != hex.EncodeToString(fingerprint[:]) {
			return &commandFailure{vault.ErrConflict, "Source item changed since transfer planning; no newer contents were copied"}
		}
		if job.Containers[0].OrganizationID != "" {
			var ids []string
			for _, id := range entry.Containers {
				ids = append(ids, mapping[id])
			}
			raw["collectionIds"], _ = json.Marshal(ids)
			raw["folderId"] = json.RawMessage("null")
		} else {
			raw["folderId"], _ = json.Marshal(mapping[entry.Containers[0]])
			raw["organizationId"] = json.RawMessage("null")
			raw["collectionIds"] = json.RawMessage("[]")
		}
		if err := p.cloneItem(ctx, entry.SourceID, raw); err != nil {
			return err
		}
		entry.Complete = true
		if err := saveHierarchy(path, job); err != nil {
			return err
		}
	}
	job.Complete = true
	return saveHierarchy(path, job)
}

func (p *Provider) verifyContainer(ctx context.Context, c hierarchyContainer) error {
	args := []string{"get", "folder", c.TargetID}
	if c.OrganizationID != "" {
		args = []string{"get", "org-collection", c.TargetID, "--organizationid", c.OrganizationID}
	}
	raw, err := decode[map[string]json.RawMessage](p.run(ctx, args, nil))
	if err != nil {
		return err
	}
	var name string
	if json.Unmarshal(raw["name"], &name) != nil || name != c.Name {
		return vault.ErrConflict
	}
	return nil
}

func (p *Provider) ensureContainerAbsent(ctx context.Context, c hierarchyContainer) error {
	args := []string{"list", "folders"}
	if c.OrganizationID != "" {
		args = []string{"list", "org-collections", "--organizationid", c.OrganizationID}
	}
	containers, err := decode[[]folder](p.run(ctx, args, nil))
	if err != nil {
		return err
	}
	for _, existing := range containers {
		if existing.Name == c.Name {
			return vault.ErrConflict
		}
	}
	return nil
}

func (p *Provider) reconcileContainer(ctx context.Context, c hierarchyContainer) (string, error) {
	if _, err := p.run(ctx, []string{"sync"}, nil); err != nil {
		return "", err
	}
	args := []string{"list", "folders"}
	if c.OrganizationID != "" {
		args = []string{"list", "org-collections", "--organizationid", c.OrganizationID}
	}
	containers, err := decode[[]folder](p.run(ctx, args, nil))
	if err != nil {
		return "", err
	}
	var id string
	for _, candidate := range containers {
		if candidate.Name == c.Name {
			if id != "" {
				return "", vault.ErrConflict
			}
			id = candidate.ID
		}
	}
	if id == "" {
		return "", &commandFailure{vault.ErrUnavailable, "Container creation is unresolved; automatic reconciliation retained the job without repeating creation"}
	}
	return id, nil
}

func (p *Provider) hierarchyLock(ctx context.Context) (func(), error) {
	lock := flock.New(filepath.Join(p.DataDir, "lazypass-hierarchy.lock"))
	ok, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, vault.ErrUnavailable
	}
	return func() { _ = lock.Unlock() }, nil
}
