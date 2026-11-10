package bitwarden

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"lazypass/internal/vault"
)

func TestCollectionDeletionLive(t *testing.T) {
	dir := os.Getenv("LAZYPASS_BW_DELETION_DIR")
	if dir == "" {
		t.Skip("set deletion fixture connection explicitly")
	}
	p := New(dir, os.Getenv("LAZYPASS_BW_DELETION_SERVER"), func() (string, error) { return "", errors.New("use password-file unlock") }, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	output, err := p.run(ctx, []string{"unlock", "--passwordfile", os.Getenv("LAZYPASS_BW_DELETION_PASSWORD_FILE"), "--raw"}, nil)
	if err != nil {
		t.Fatal("fixture unlock failed")
	}
	p.session = string(bytes.TrimSpace(output))
	clear(output)
	defer p.Lock()
	before, err := p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("baseline failed")
	}
	if len(before.organizations) != 1 {
		t.Fatal("requires one organization")
	}
	org := before.organizations[0].ID
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	prefix := "lazypass-delete-" + hex.EncodeToString(random[:])
	var groups, items []string
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Minute)
		defer stop()
		current, err := p.deletionSnapshot(cleanup)
		if err != nil {
			t.Error("cleanup listing failed")
			return
		}
		active := make(map[string]bool)
		for _, entry := range current.excluded {
			active[entry.ID] = true
		}
		presentGroups := make(map[string]bool)
		for _, group := range current.collections {
			presentGroups[group.ID] = true
		}
		for _, id := range items {
			if active[id] {
				if _, err := p.run(cleanup, []string{"delete", "item", id}, nil); err != nil {
					t.Error("fixture item cleanup failed")
					return
				}
			}
		}
		for i := len(groups) - 1; i >= 0; i-- {
			if presentGroups[groups[i]] {
				if _, err := p.run(cleanup, []string{"delete", "org-collection", groups[i], "--organizationid", org}, nil); err != nil {
					t.Error("fixture collection cleanup failed")
					return
				}
			}
		}
		after, err := p.deletionSnapshot(cleanup)
		if err != nil {
			t.Error("cleanup verification failed")
			return
		}
		for _, g := range after.collections {
			for _, id := range groups {
				if g.ID == id {
					t.Error("fixture collection remains")
				}
			}
		}
		present := make(map[string]bool)
		for _, i := range after.excluded {
			present[i.ID] = true
		}
		for _, i := range after.items {
			present[i.ID] = true
		}
		for _, id := range items {
			if present[id] {
				t.Error("fixture entry remains active")
			}
		}
		for _, i := range before.excluded {
			if !present[i.ID] {
				t.Error("existing entry missing")
			}
		}
		for _, g := range before.collections {
			found := false
			for _, a := range after.collections {
				if a.ID == g.ID && a.Name == g.Name {
					found = true
				}
			}
			if !found {
				t.Error("existing collection changed")
			}
		}
	}()
	s := before
	for _, name := range []string{prefix, prefix + "/nested", prefix + "-outside"} {
		g, err := p.ensureCollection(ctx, &s, org, name)
		if err != nil {
			t.Fatal("fixture collection creation failed")
		}
		groups = append(groups, g.ID)
	}
	create := func(name string, members []string) string {
		t.Helper()
		payload := itemPayload(vault.Entry{Type: vault.NoteEntry, Notes: "Public disposable deletion fixture"}, name, "")
		payload["organizationId"], payload["collectionIds"] = org, members
		i, err := decode[item](p.submit(ctx, []string{"create", "item"}, payload))
		if err != nil || i.ID == "" {
			t.Fatal("fixture item creation failed")
		}
		items = append(items, i.ID)
		return i.ID
	}
	create(prefix+"-shared", []string{groups[0], groups[1], groups[2]})
	create(prefix+"-nested", []string{groups[1]})
	target := vault.Path{"Organizations", org, groups[0]}
	plan, err := p.PlanDelete(ctx, target)
	if err != nil || plan.Entries != 2 || plan.Containers != 2 {
		t.Fatal("unexpected deletion scope")
	}
	create(prefix+"-scope-change", []string{groups[0]})
	if err := p.ExecuteDelete(ctx, plan); !errors.Is(err, vault.ErrConflict) {
		t.Fatal("scope change not rejected")
	}
	plan, err = p.PlanDelete(ctx, target)
	if err != nil {
		t.Fatal("replanning failed")
	}
	if err := p.ExecuteDelete(ctx, plan); err != nil {
		t.Fatalf("fixture deletion failed: %v", err)
	}
	after, err := p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("result sync failed")
	}
	outside := false
	for _, g := range after.collections {
		if g.ID == groups[2] {
			outside = true
		}
		if g.ID == groups[0] || g.ID == groups[1] {
			t.Fatal("target collection remains")
		}
	}
	if !outside {
		t.Fatal("outside collection deleted")
	}
	for _, i := range after.excluded {
		for _, id := range items {
			if i.ID == id {
				t.Fatal("shared fixture remains active")
			}
		}
	}
	t.Log("live scope-change rejection, nested deletion, global shared-item deletion and outside collection preservation passed")
}
