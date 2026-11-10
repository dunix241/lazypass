package otp

import (
	"context"
	"os"
	"testing"
)

func TestMigrationImageDiagnostic(t *testing.T) {
	path := os.Getenv("LAZYPASS_MIGRATION_IMAGE")
	if path == "" {
		t.Skip("set diagnostic image path")
	}
	values, err := ReadQRImage(context.Background(), path)
	if err != nil {
		t.Fatalf("image stage: %v", err)
	}
	t.Logf("decoded symbols: %d", len(values))
	for _, value := range values {
		page, err := ParseMigration(value)
		if err != nil {
			t.Fatal("migration stage rejected payload")
		}
		t.Logf("migration accepted: version=%d pages=%d index=%d records=%d", page.Version, page.Size, page.Index, len(page.Records))
	}
}
