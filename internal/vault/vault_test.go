package vault

import (
	"errors"
	"strings"
	"testing"
)

func TestMultilineNotesRoundTrip(t *testing.T) {
	entry := Entry{Path: Path{"fixture"}, Password: "fixture", Notes: "first\nsecond\r\nthird"}
	encoded, err := EncodeEntry(entry)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeEntry(entry.Path, encoded)
	if err != nil || decoded.Notes != entry.Notes {
		t.Fatalf("notes round trip: %v", err)
	}
}

func TestTypedEntriesRoundTrip(t *testing.T) {
	for _, kind := range []EntryType{CardEntry, IdentityEntry, NoteEntry, SSHKeyEntry} {
		entry := Entry{Type: kind, Path: Path{"fixture"}, Notes: "multiline\nnotes", Fields: map[string]string{"privateKey": "first\nsecond", "number": "1234"}}
		encoded, err := EncodeEntry(entry)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeEntry(entry.Path, encoded)
		if err != nil || decoded.Type != kind || decoded.Notes != entry.Notes || decoded.Fields["privateKey"] != entry.Fields["privateKey"] {
			t.Fatalf("typed round trip: %v", err)
		}
	}
}

func TestPathValidation(t *testing.T) {
	for _, path := range []Path{{""}, {"."}, {".."}, {"a/b"}, {"a\\b"}, {"a.gpg"}, {"a\n"}} {
		if !errors.Is(path.Validate(), ErrInvalidPath) {
			t.Fatalf("path %q was accepted", path)
		}
	}
	if err := (Path{"Personal", "Mail"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestEntryCodecRoundTrip(t *testing.T) {
	want := Entry{Path: Path{"Personal", "mail"}, Password: "secret", Username: "me", URL: "https://example.test", Notes: "note", Fields: map[string]string{"Account": "42"}}
	encoded, err := EncodeEntry(want)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, want.Path.String()) {
		t.Fatal("path appeared in entry payload")
	}
	got, err := DecodeEntry(want.Path, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Password != want.Password || got.Username != want.Username || got.URL != want.URL || got.Notes != want.Notes || got.Fields["Account"] != "42" {
		t.Fatalf("decoded entry = %#v", got)
	}
}

func TestDecodeMalformedEntryDoesNotExposeSecret(t *testing.T) {
	_, err := DecodeEntry(Path{"entry"}, "super-secret\ninvalid metadata")
	if err == nil {
		t.Fatal("expected malformed entry error")
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatal("secret leaked in error")
	}
}

func TestEntryCodecRejectsMultilineValues(t *testing.T) {
	for _, entry := range []Entry{
		{Path: Path{"entry"}, Password: "first\nsecond"},
		{Path: Path{"entry"}, Username: "name\rvalue"},
		{Path: Path{"entry"}, Fields: map[string]string{"Account": "one\ntwo"}},
	} {
		if _, err := EncodeEntry(entry); err == nil {
			t.Fatalf("accepted multiline entry: %#v", entry)
		}
	}
}
