package otp

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/boombuler/barcode/qr"
)

var ErrQRExport = errors.New("cannot export QR image: choose a new writable file path")

const qrQuietZone = 4

type scaledQR struct {
	image.Image
	scale int
}

func (s scaledQR) Bounds() image.Rectangle {
	b := s.Image.Bounds()
	return image.Rect(0, 0, b.Dx()*s.scale, b.Dy()*s.scale)
}

func (s scaledQR) At(x, y int) color.Color {
	b := s.Image.Bounds()
	return s.Image.At(b.Min.X+x/s.scale, b.Min.Y+y/s.scale)
}

// QRImage includes the four-module quiet zone required for scanning.
func QRImage(ctx context.Context, value, label string) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	uri, err := Normalize(value, label)
	if err != nil || uri == "" {
		return nil, ErrInvalid
	}
	code, err := qr.Encode(uri, qr.M, qr.Auto)
	if err != nil {
		return nil, ErrInvalid
	}
	bounds := code.Bounds()
	img := image.NewGray(image.Rect(0, 0, bounds.Dx()+2*qrQuietZone, bounds.Dy()+2*qrQuietZone))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			img.Set(x+qrQuietZone, y+qrQuietZone, code.At(x+bounds.Min.X, y+bounds.Min.Y))
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return img, nil
}

func ExportQR(ctx context.Context, path string, img image.Image) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if img == nil {
		return ErrInvalid
	}
	output := scaledQR{Image: img, scale: 8}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return ErrQRExport
	}
	success := false
	defer func() {
		if !success {
			_ = os.Remove(path)
		}
	}()
	err = png.Encode(file, output)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return ErrQRExport
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	success = true
	return nil
}
