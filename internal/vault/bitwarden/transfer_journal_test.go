package bitwarden

import (
	"encoding/json"
	"os"
	"testing"
)

func TestTransferJournalSurvivesRestartAndRejectsOtherAccount(t *testing.T) {
	dir := t.TempDir()
	first := New(dir, "", nil, nil)
	var signature [32]byte
	signature[0] = 1
	first.cloneAttempts = map[[32]byte]*cloneAttempt{signature: {TargetID: "confirmed-id", BeforeIDs: []string{"old-id"}}}
	if err := first.saveCloneJournal("account-a"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(first.journalPath())
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("journal permissions")
	}
	second := New(dir, "", nil, nil)
	second.cloneAttempts = map[[32]byte]*cloneAttempt{}
	if err := second.loadCloneJournal("account-a"); err != nil {
		t.Fatal(err)
	}
	if second.cloneAttempts[signature].TargetID != "confirmed-id" {
		t.Fatal("destination identity lost")
	}
	if err := second.loadCloneJournal("account-b"); err == nil {
		t.Fatal("account mismatch accepted")
	}
}

func TestJournalRejectsTrailingDataAndUnexpectedDestinationFields(t *testing.T) {
	p := New(t.TempDir(), "", nil, nil)
	p.cloneAttempts = map[[32]byte]*cloneAttempt{}
	for _, data := range []string{
		`{"version":1,"account":"account-a","attempts":{}} {}`,
		`{"version":1,"account":"account-a","attempts":{"0000000000000000000000000000000000000000000000000000000000000000":{"destination":{"password":"secret"}}}}`,
	} {
		if err := os.WriteFile(p.journalPath(), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if p.loadCloneJournal("account-a") == nil {
			t.Fatal("invalid journal accepted")
		}
	}
}

func TestCloneContentComparisonDetectsDestinationEdits(t *testing.T) {
	expected := map[string]json.RawMessage{"name": json.RawMessage(`"fixture"`), "notes": json.RawMessage(`"original"`)}
	actual := map[string]json.RawMessage{"name": json.RawMessage(`"fixture"`), "notes": json.RawMessage(`"edited externally"`)}
	if cloneContentsMatch(expected, actual) {
		t.Fatal("external edit accepted")
	}
	actual["notes"] = expected["notes"]
	if !cloneContentsMatch(expected, actual) {
		t.Fatal("matching clone rejected")
	}
	actual["deletedDate"] = json.RawMessage(`"2026-11-12"`)
	if cloneContentsMatch(expected, actual) {
		t.Fatal("deleted clone accepted")
	}
}
