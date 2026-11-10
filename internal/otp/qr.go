package otp

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"

	"github.com/makiuchi-d/gozxing"
	qr "github.com/makiuchi-d/gozxing/multi/qrcode"
)

const (
	maxImageBytes  = 16 << 20
	maxImagePixels = 8_000_000
)

var (
	ErrQRFile     = errors.New("cannot read QR image: choose a regular PNG or JPEG file")
	ErrQRImage    = errors.New("invalid QR image or image exceeds the size limit")
	ErrQRMissing  = errors.New("no readable QR code found in the image")
	ErrQRMultiple = errors.New("multiple QR codes found: choose an image containing one code")
)

// ImportQR never returns decoder or filesystem errors that may contain input.
func ImportQR(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", ErrQRFile
	}
	if info.Size() > maxImageBytes {
		return "", ErrQRImage
	}
	file, err := os.Open(path)
	if err != nil {
		return "", ErrQRFile
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", ErrQRFile
	}
	if info.Size() > maxImageBytes {
		return "", ErrQRImage
	}
	data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	defer clear(data)
	if err != nil || len(data) > maxImageBytes {
		return "", ErrQRImage
	}
	return DecodeQR(ctx, data)
}

// DecodeQR applies the same image bounds and validation to file and clipboard data.
func DecodeQR(ctx context.Context, data []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(data) > maxImageBytes {
		return "", ErrQRImage
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || config.Width <= 0 || config.Height <= 0 || config.Width > maxImagePixels/config.Height {
		return "", ErrQRImage
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", ErrQRImage
	}
	bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", ErrQRImage
	}
	results, err := qr.NewQRCodeMultiReader().DecodeMultiple(bitmap, nil)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err != nil || len(results) == 0 {
		return "", ErrQRMissing
	}
	if len(results) != 1 {
		return "", ErrQRMultiple
	}
	value := results[0].GetText()
	key, err := Parse(value)
	if err != nil || key.uri == "" {
		return "", ErrInvalid
	}
	return key.uri, nil
}
