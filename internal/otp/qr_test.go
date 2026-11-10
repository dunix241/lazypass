package otp

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
)

func qrFixture(t *testing.T, value string) image.Image {
	t.Helper()
	code, err := qr.Encode(value, qr.M, qr.Auto)
	if err != nil {
		t.Fatal(err)
	}
	scaled, err := barcode.Scale(code, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 360, 360))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(30, 30, 330, 330), scaled, image.Point{}, draw.Src)
	return canvas
}

func TestQRImportImageFormatsAndAmbiguity(t *testing.T) {
	const uri = "otpauth://totp/PublicFixture?secret=JBSWY3DPEHPK3PXP"
	for _, format := range []string{"png", "jpeg"} {
		path := filepath.Join(t.TempDir(), "fixture."+format)
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if format == "png" {
			err = png.Encode(file, qrFixture(t, uri))
		} else {
			err = jpeg.Encode(file, qrFixture(t, uri), &jpeg.Options{Quality: 95})
		}
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		got, err := ImportQR(context.Background(), path)
		if err != nil || got != uri {
			t.Fatalf("%s public fixture import failed: %v", format, err)
		}
	}
	for _, tc := range []struct {
		name string
		img  image.Image
		want error
	}{
		{"empty", image.NewGray(image.Rect(0, 0, 100, 100)), ErrQRMissing},
		{"unsupported", qrFixture(t, "https://example.invalid"), ErrInvalid},
		{"multiple", multipleQRFixture(t, uri), ErrQRMultiple},
	} {
		path := filepath.Join(t.TempDir(), tc.name+".png")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, tc.img)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ImportQR(context.Background(), path); err != tc.want {
			t.Fatalf("%s error = %v", tc.name, err)
		}
	}
}

func multipleQRFixture(t *testing.T, uri string) image.Image {
	canvas := image.NewRGBA(image.Rect(0, 0, 760, 360))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(0, 0, 360, 360), qrFixture(t, uri), image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(400, 0, 760, 360), qrFixture(t, uri+"&period=60"), image.Point{}, draw.Src)
	return canvas
}

func TestQRImportFileErrorsAndCancellation(t *testing.T) {
	if _, err := ImportQR(context.Background(), t.TempDir()); err != ErrQRFile {
		t.Fatal("directory accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ImportQR(ctx, "missing"); err != context.Canceled {
		t.Fatal("cancellation ignored")
	}
	path := filepath.Join(t.TempDir(), "large")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = file.Truncate(maxImageBytes + 1)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ImportQR(context.Background(), path); err != ErrQRImage {
		t.Fatal("oversize image accepted")
	}
}
