package bitwarden

import (
	"context"
	"lazypass/internal/vault"
	"os"
	"testing"
)

func TestHierarchyJournalRetainsCompletedStepsAcrossRestart(t *testing.T) {
	p := New(t.TempDir(), "", nil, &currentMemberRunner{})
	path := p.hierarchyPath(vault.Path{"Folders", "source"}, vault.Path{"Folders"}, "Copy")
	job := &hierarchyJob{Version: 1, Account: transferAccount(p.Server, "account-id"), Containers: []hierarchyContainer{{SourceID: "source", Name: "Copy", TargetID: "confirmed", Attempted: true}}, Items: []hierarchyItem{{SourceID: "item", Containers: []string{"source"}, Complete: true}}}
	if err := saveHierarchy(path, job); err != nil {
		t.Fatal(err)
	}
	restarted := New(p.DataDir, "", nil, &currentMemberRunner{})
	loaded, err := restarted.loadHierarchy(context.Background(), path)
	if err != nil || loaded == nil || loaded.Containers[0].TargetID != "confirmed" || !loaded.Items[0].Complete {
		t.Fatal("hierarchy progress lost", err)
	}
	if err := restarted.runHierarchyCopy(context.Background(), path, loaded); err != nil {
		t.Fatal(err)
	}
	if job, err := restarted.loadHierarchy(context.Background(), path); err != nil || job == nil || !job.Complete {
		t.Fatal("completed receipt not retained")
	}
}

func TestCancelledHierarchyIsPausedDurably(t *testing.T) {
	p := New(t.TempDir(), "", nil, &currentMemberRunner{})
	path := p.hierarchyPath(vault.Path{"Folders", "source"}, vault.Path{"Folders"}, "Copy")
	job := &hierarchyJob{Version: 1, Account: transferAccount(p.Server, "account-id"), Containers: []hierarchyContainer{{SourceID: "source", Name: "Copy"}}}
	if err := saveHierarchy(path, job); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if p.runHierarchyCopy(ctx, path, job) == nil {
		t.Fatal("cancel ignored")
	}
	loaded, err := p.loadHierarchy(context.Background(), path)
	if err != nil || !loaded.Paused {
		t.Fatal("cancellation not retained", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("cancelled job lost")
	}
}

func TestHierarchyPlanDeduplicatesSharedItems(t *testing.T) {
	p := New(t.TempDir(), "", nil, &currentMemberRunner{})
	s := snapshot{excluded: []item{{ID: "shared", CollectionIDs: []string{"a", "b"}}, {ID: "shared", CollectionIDs: []string{"a", "b"}}}}
	job, err := p.newHierarchy(context.Background(), s, []containerTransfer{{id: "a", organizationID: "org", target: "Copy"}, {id: "b", organizationID: "org", target: "Copy/Child"}})
	if err != nil || len(job.Items) != 1 || len(job.Items[0].Containers) != 2 {
		t.Fatal("shared source duplicated", err)
	}
}
