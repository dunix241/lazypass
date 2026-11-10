package otp

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestQRExportRoundTripAndNoOverwrite(t *testing.T) {
	ctx := context.Background()
	const uri = "otpauth://totp/PublicFixture?secret=JBSWY3DPEHPK3PXP&period=60&digits=8"
	img, err := QRImage(ctx, uri, "ignored")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture.png")
	if err := ExportQR(ctx, path, img); err != nil {
		t.Fatal(err)
	}
	got, err := ImportQR(ctx, path)
	if err != nil || got != uri {
		t.Fatal("exported QR failed round trip")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ExportQR(ctx, path, img); err != ErrQRExport {
		t.Fatal("existing file overwritten")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("existing file changed")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("QR export is not private")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := QRImage(ctx, uri, "ignored"); err != context.Canceled {
		t.Fatal("generation ignored cancellation")
	}
}
