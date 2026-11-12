package bitwarden

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"lazypass/internal/vault"
)

type lostCreateResponseRunner struct {
	Runner
	hide    bool
	creates int
}

func (r *lostCreateResponseRunner) Run(ctx context.Context, args []string, in io.Reader, env []string) ([]byte, error) {
	out, err := r.Runner.Run(ctx, args, in, env)
	if len(args) > 1 && args[0] == "create" && args[1] == "item" {
		r.creates++
		if err == nil && r.hide {
			r.hide = false
			clear(out)
			return nil, errors.New("injected lost successful response")
		}
	}
	return out, err
}

func TestOwnershipAndRecoveryLive(t *testing.T) {
	dir := os.Getenv("LAZYPASS_BW_AUTH_DIR")
	if dir == "" {
		t.Skip("set live vault test settings")
	}
	prompt := func() (string, error) {
		data, err := os.ReadFile(os.Getenv("LAZYPASS_BW_AUTH_PASSWORD_FILE"))
		defer clear(data)
		return strings.TrimRight(string(data), "\r\n"), err
	}
	r := &lostCreateResponseRunner{Runner: ExecRunner{}}
	p := New(dir, os.Getenv("LAZYPASS_BW_AUTH_SERVER"), prompt, r)
	defer p.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	before, err := p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("initial snapshot failed")
	}
	if len(before.organizations) == 0 {
		t.Fatal("organization required")
	}
	org := before.organizations[0].ID
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	prefix := "lazypass-ownership-" + hex.EncodeToString(nonce[:])
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 3*time.Minute)
		defer stop()
		s, err := p.deletionSnapshot(cleanup)
		if err != nil {
			t.Error("cleanup snapshot failed")
			return
		}
		for _, entry := range append(s.items, s.excluded...) {
			if strings.HasPrefix(entry.Name, prefix) {
				if _, err := p.run(cleanup, []string{"delete", "item", entry.ID}, nil); err != nil {
					t.Error("fixture item cleanup failed")
				}
			}
		}
		for _, c := range s.collections {
			if strings.HasPrefix(c.Name, prefix) {
				if _, err := p.run(cleanup, []string{"delete", "org-collection", c.ID, "--organizationid", org}, nil); err != nil {
					t.Error("fixture collection cleanup failed")
				}
			}
		}
		after, err := p.deletionSnapshot(cleanup)
		if err != nil {
			t.Error("cleanup verification failed")
			return
		}
		remaining := map[string]item{}
		for _, entry := range append(after.items, after.excluded...) {
			remaining[entry.ID] = entry
			if strings.HasPrefix(entry.Name, prefix) {
				t.Error("fixture item remains")
			}
		}
		for _, c := range after.collections {
			if strings.HasPrefix(c.Name, prefix) {
				t.Error("fixture collection remains")
			}
		}
		for _, old := range append(before.items, before.excluded...) {
			current, ok := remaining[old.ID]
			if !ok || current.Name != old.Name || current.OrganizationID != old.OrganizationID || current.FolderID != old.FolderID || strings.Join(current.CollectionIDs, ",") != strings.Join(old.CollectionIDs, ",") {
				t.Error("existing metadata changed")
			}
		}
	}()
	source, err := decode[collection](p.submit(ctx, []string{"create", "org-collection", "--organizationid", org}, collectionClonePayload(containerTransfer{organizationID: org, target: prefix + "-source"})))
	if err != nil || source.ID == "" {
		t.Fatal("source collection creation failed")
	}
	if _, err := p.deletionSnapshot(ctx); err != nil {
		t.Fatal("collection sync failed")
	}
	if err := p.TransferFolder(ctx, vault.Path{"Organizations", org, source.ID}, vault.Path{"Organizations", org}, prefix+"-copy", false); err != nil {
		t.Fatal("collection copy failed", err)
	}
	s, err := p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("copy sync failed")
	}
	copyID := ""
	for _, c := range s.collections {
		if c.Name == prefix+"-copy" {
			copyID = c.ID
		}
	}
	if copyID == "" {
		t.Fatal("copied collection missing")
	}
	raw, err := decode[map[string]json.RawMessage](p.run(ctx, []string{"get", "org-collection", copyID, "--organizationid", org}, nil))
	if err != nil {
		t.Fatal("access read failed")
	}
	var users, groups []json.RawMessage
	if json.Unmarshal(raw["users"], &users) != nil || json.Unmarshal(raw["groups"], &groups) != nil || len(users)+len(groups) != 0 {
		t.Fatal("collection copy added sharing assignments")
	}
	t.Log("collection copy without sharing assignments passed")
	name := prefix + "-personal"
	if err := p.Create(ctx, vault.Entry{Path: vault.Path{"Folders", noFolderID, name}, Password: "public-fixture", Notes: "preserved"}); err != nil {
		t.Fatal("personal fixture creation failed")
	}
	s, err = p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("fixture sync failed")
	}
	id := ""
	for _, entry := range s.items {
		if entry.Name == name {
			id = entry.ID
		}
	}
	if id == "" {
		t.Fatal("personal fixture missing")
	}
	path := vault.Path{"Folders", noFolderID, id}
	destination := vault.Path{"Organizations", org, copyID}
	r.hide = true
	creates := r.creates
	if err := p.Transfer(ctx, path, destination, prefix+"-org-clone", false); err != nil {
		t.Fatal("lost response recovery failed", err)
	}
	if r.creates != creates+1 {
		t.Fatal("creation repeated after lost response")
	}
	s, err = p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("recovery sync failed")
	}
	count := 0
	for _, entry := range s.excluded {
		if entry.Name == prefix+"-org-clone" && entry.OrganizationID == org {
			count++
		}
	}
	if count != 1 {
		t.Fatal("recovered copy count differs from one")
	}
	t.Log("ownership-changing copy and lost-response reconciliation passed")
	if err := p.Transfer(ctx, path, destination, name, true); err != nil {
		t.Fatal("native ownership move failed", err)
	}
	s, err = p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("ownership sync failed")
	}
	found := false
	for _, entry := range s.excluded {
		if entry.ID == id && entry.OrganizationID == org {
			found = true
		}
	}
	if !found {
		t.Fatal("ownership move did not preserve item ID")
	}
	t.Log("native ownership move preserved item ID")
}
