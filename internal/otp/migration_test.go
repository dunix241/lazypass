package otp

import (
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"strings"
	"testing"
)

// Literal protobuf bytes are independent of the production decoder/encoder.
func migrationFixture(t *testing.T, index byte) string {
	t.Helper()
	// Public secret "12345678901234567890", name "alice", issuer "Example".
	data, err := hex.DecodeString("0a2a0a1431323334353637383930313233343536373839301205616c6963651a074578616d706c65200128013002100118022000282a")
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-3] = index
	return "otpauth-migration://offline?data=" + url.QueryEscape(base64.StdEncoding.EncodeToString(data))
}

func TestMigrationVersionTwoUsesSameRecordAndBatchFormat(t *testing.T) {
	value := migrationFixture(t, 0)
	u, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(u.Query().Get("data"))
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-7] = 2
	value = "otpauth-migration://offline?data=" + url.QueryEscape(base64.StdEncoding.EncodeToString(data))
	page, err := ParseMigration(value)
	if err != nil || page.Version != 2 || !strings.Contains(page.Records[0].URI, "secret=") {
		t.Fatal("version2 public fixture rejected")
	}
	var batch MigrationBatch
	if err := batch.Add(page); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationBatchReordersAndRejectsConflicts(t *testing.T) {
	first, err := ParseMigration(migrationFixture(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseMigration(migrationFixture(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	if first.Records[0].Name != "alice" || first.Records[0].Issuer != "Example" || first.Records[0].URI == "" {
		t.Fatal("public fixture mapping failed")
	}
	var batch MigrationBatch
	if err := batch.Add(second); err != nil {
		t.Fatal(err)
	}
	if len(batch.Missing()) != 1 || batch.Missing()[0] != 0 {
		t.Fatal("missing page not identified")
	}
	if _, err := batch.Records(); err != ErrMigrationBatch {
		t.Fatal("incomplete batch accepted")
	}
	if err := batch.Add(first); err != nil {
		t.Fatal(err)
	}
	if err := batch.Add(first); err != nil {
		t.Fatal("identical repeat rejected")
	}
	records, err := batch.Records()
	if err != nil || len(records) != 2 {
		t.Fatal("complete batch failed")
	}
	first.ID++
	if err := batch.Add(first); err != ErrMigrationBatch {
		t.Fatal("mixed batch accepted")
	}
}

func TestMigrationImagePagesAreAddedAtomically(t *testing.T) {
	first, err := ParseMigration(migrationFixture(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseMigration(migrationFixture(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	second.ID++
	var batch MigrationBatch
	if err := batch.AddPages([]MigrationPage{first, second}); err != ErrMigrationBatch {
		t.Fatal("mixed image accepted")
	}
	if loaded, total := batch.Progress(); loaded != 0 || total != 0 {
		t.Fatal("rejected image partially changed batch")
	}
	if err := batch.Add(first); err != nil {
		t.Fatal(err)
	}
	if err := batch.AddPages([]MigrationPage{second}); err != ErrMigrationBatch {
		t.Fatal("mixed page accepted")
	}
	if loaded, total := batch.Progress(); loaded != 1 || total != 2 {
		t.Fatal("existing progress changed")
	}
}

func TestMigrationMalformedAndUnsupportedRecords(t *testing.T) {
	for _, value := range []string{"", "otpauth-migration://offline?data=bad", migrationFixture(t, 2), migrationFixture(t, 0) + "&data=other", migrationFixture(t, 0) + "#"} {
		if _, err := ParseMigration(value); err != ErrMigration {
			t.Fatal("malformed export accepted")
		}
	}
	data, err := hex.DecodeString("0a0131200128013001")
	if err != nil {
		t.Fatal(err)
	}
	record, err := parseMigrationRecord(data)
	if err != nil || record.Unsupported != "HOTP is not supported" || record.URI != "" {
		t.Fatal("HOTP was converted to TOTP")
	}
	if _, err := parseMigrationRecord([]byte{0x0a, 0xff}); err != ErrMigration {
		t.Fatal("truncated wire accepted")
	}
}
