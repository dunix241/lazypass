package bitwarden

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"lazypass/internal/vault"
)

// Opt-in test mutates only uniquely named fixtures, never existing objects.
func TestCollectionCreationLive(t *testing.T) {
	dir := os.Getenv("LAZYPASS_BW_CREATION_DIR")
	if dir == "" {
		t.Skip("set creation test directory, server and password file")
	}
	p := New(dir, os.Getenv("LAZYPASS_BW_CREATION_SERVER"), func() (string, error) {
		data, err := os.ReadFile(os.Getenv("LAZYPASS_BW_CREATION_PASSWORD_FILE"))
		defer clear(data)
		return strings.TrimRight(string(data), "\r\n"), err
	}, nil)
	defer p.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	before, err := p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("initial snapshot failed")
	}
	if len(before.organizations) != 1 {
		t.Fatal("test requires exactly one existing organization")
	}
	org := before.organizations[0].ID
	var token [12]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	name := "lazypass-test-" + hex.EncodeToString(token[:])
	// Cleanup runs while session and context are still available. Match both
	// exact fixture names and IDs absent from the original snapshot.
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Minute)
		defer stop()
		if _, err := p.run(cleanup, []string{"sync"}, nil); err != nil {
			t.Error("cleanup sync failed")
			return
		}
		p.metadata = nil
		s, err := p.snapshot(cleanup)
		if err != nil {
			t.Error("cleanup snapshot failed")
			return
		}
		for _, item := range s.excluded {
			if item.OrganizationID == org && item.Name == name+"-entry" {
				if _, err := p.run(cleanup, []string{"delete", "item", item.ID}, nil); err != nil {
					t.Error("fixture entry cleanup failed")
				}
			}
		}
		for _, group := range s.collections {
			if group.OrganizationID == org && (group.Name == name || group.Name == name+"/child") {
				if _, err := p.run(cleanup, []string{"delete", "org-collection", group.ID, "--organizationid", org}, nil); err != nil {
					t.Error("fixture collection cleanup failed")
				}
			}
		}
		after, err := p.deletionSnapshot(cleanup)
		if err != nil {
			t.Error("post-cleanup verification failed")
			return
		}
		for _, old := range before.collections {
			found := false
			for _, current := range after.collections {
				if current.ID == old.ID && current.Name == old.Name {
					found = true
					break
				}
			}
			if !found {
				t.Error("existing collection changed")
			}
		}
		ids := make(map[string]bool)
		for _, item := range after.excluded {
			ids[item.ID] = true
		}
		for _, item := range after.items {
			ids[item.ID] = true
		}
		for _, item := range append(before.items, before.excluded...) {
			if !ids[item.ID] {
				t.Error("existing entry missing")
			}
		}
	}()
	path := vault.Path{"Organizations", org, name, "child", name + "-entry"}
	if err := p.ValidateDestination(ctx, path); err != nil {
		t.Fatal("destination preflight failed")
	}
	if len(p.metadata.collections) != len(before.collections) {
		t.Fatal("preflight mutated collections")
	}
	if err := p.Create(ctx, vault.Entry{Path: path, Type: vault.NoteEntry, Notes: "Public lazypass integration fixture"}); err != nil {
		t.Fatalf("nested fixture creation: %v", err)
	}
	if _, err := p.Sync(ctx); err != nil {
		t.Fatal("post-creation sync failed")
	}
	p.metadata = nil
	s, err := p.snapshot(ctx)
	if err != nil {
		t.Fatal("refresh failed")
	}
	var groupID string
	for _, group := range s.collections {
		if group.OrganizationID == org && group.Name == name+"/child" {
			groupID = group.ID
		}
	}
	if groupID == "" {
		t.Fatal("nested collection not created")
	}
	var itemID string
	for _, item := range s.excluded {
		if item.OrganizationID == org && item.Name == name+"-entry" {
			itemID = item.ID
		}
	}
	if itemID == "" {
		t.Fatal("fixture entry not created")
	}
	entry, err := p.Read(ctx, vault.Path{"Organizations", org, groupID, itemID})
	if err != nil || entry.Notes != "Public lazypass integration fixture" {
		t.Fatal("fixture round trip failed")
	}
	folderID, err := p.ensureFolder(ctx, &s, vault.Path{name + "-personal"})
	if err != nil {
		t.Fatal("fixture personal folder creation failed")
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		folders, err := decode[[]folder](p.run(cleanup, []string{"list", "folders"}, nil))
		if err != nil {
			t.Error("personal folder cleanup listing failed")
			return
		}
		for _, folder := range folders {
			if folder.ID == folderID {
				if _, err := p.run(cleanup, []string{"delete", "folder", folderID}, nil); err != nil {
					t.Error("personal folder cleanup failed")
				}
			}
		}
	}()
	raw, err := decode[map[string]any](p.run(ctx, []string{"get", "item", itemID}, nil))
	if err != nil {
		t.Fatal("fixture assignment read failed")
	}
	raw["folderId"] = folderID
	if _, err := p.submit(ctx, []string{"edit", "item", itemID}, raw); err != nil {
		t.Fatal("fixture folder assignment failed")
	}
	if _, err := p.Sync(ctx); err != nil {
		t.Fatal("assignment sync failed")
	}
	folderPath := vault.Path{"Folders", folderID, itemID}
	fromFolder, err := p.Read(ctx, folderPath)
	if err != nil {
		t.Fatal("shared item missing in folder view")
	}
	fromFolder.Notes = "Public mixed membership fixture"
	if err := p.Write(ctx, fromFolder); err != nil {
		t.Fatal("folder-view edit failed")
	}
	fromCollection, err := p.Read(ctx, vault.Path{"Organizations", org, groupID, itemID})
	if err != nil || fromCollection.Notes != fromFolder.Notes {
		t.Fatal("views disagree after edit")
	}
	if err := p.RemoveFolder(ctx, vault.Path{"Folders", folderID}); err != nil {
		t.Fatal("personal folder preserving removal failed")
	}
	fromNoFolder, err := p.Read(ctx, vault.Path{"Folders", noFolderID, itemID})
	if err != nil || fromNoFolder.Notes != fromFolder.Notes {
		t.Fatal("folder removal lost shared entry")
	}
	if _, err := p.Read(ctx, vault.Path{"Organizations", org, groupID, itemID}); err != nil {
		t.Fatal("folder removal changed collection membership")
	}
	fromNoFolder.Notes = "Public lazypass integration fixture"
	if err := p.Write(ctx, fromNoFolder); err != nil {
		t.Fatal("No Folder edit failed")
	}
	// Move the fixture's membership to the parent, then remove only that parent.
	parentID := ""
	for _, group := range s.collections {
		if group.OrganizationID == org && group.Name == name {
			parentID = group.ID
		}
	}
	if parentID == "" {
		t.Fatal("fixture parent missing")
	}
	if _, err := p.submit(ctx, []string{"edit", "item-collections", itemID, "--organizationid", org}, []string{parentID}); err != nil {
		t.Fatal("fixture membership setup failed")
	}
	if err := p.RemoveFolder(ctx, vault.Path{"Organizations", org, parentID}); err != nil {
		t.Fatal("preserving collection removal failed")
	}
	s, err = p.deletionSnapshot(ctx)
	if err != nil {
		t.Fatal("preservation verification failed")
	}
	childExists := false
	for _, group := range s.collections {
		if group.ID == parentID {
			t.Fatal("removed parent still present")
		}
		if group.ID == groupID {
			childExists = true
		}
	}
	if !childExists {
		t.Fatal("nested collection removed")
	}
	retained := false
	for _, item := range s.excluded {
		if item.ID == itemID {
			retained = true
			if len(item.CollectionIDs) != 0 {
				t.Fatal("removed membership retained")
			}
		}
	}
	if !retained {
		t.Fatal("collection removal deleted fixture entry")
	}
	entry, err = p.Read(ctx, vault.Path{"Organizations", org, "uncollected", itemID})
	if err != nil || entry.Notes != "Public lazypass integration fixture" {
		t.Fatal("uncollected fixture cannot be read")
	}
	t.Log("nested collection creation and entry round trip succeeded; cleanup verified on exit")
}
