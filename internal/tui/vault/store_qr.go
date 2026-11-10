package vault

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"lazypass/internal/clipboard"

	"lazypass/internal/otp"
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

func (s *StoreForm) handleQRKey(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		if s.qrCancel != nil {
			s.qrCancel()
			s.qrCancel = nil
		}
		s.qrImport = false
		s.qrPath = ""
	case tcell.KeyEnter:
		if !s.saving {
			s.importQR()
		}
	case tcell.KeyCtrlV:
		if !s.saving {
			s.importClipboardQR()
		}
	default:
		if !s.saving {
			s.editInput(event)
		}
	}
}

func (s *StoreForm) importQR() {
	path, err := qrImagePath(s.qrPath)
	if err != nil {
		s.notify(shared.Error, err.Error())
		return
	}
	s.runQRImport(func(ctx context.Context) (string, error) { return otp.ImportQR(ctx, path) })
}

func qrImagePath(value string) (string, error) {
	path := strings.TrimSpace(value)
	if len(path) >= 2 && ((path[0] == '"' && path[len(path)-1] == '"') || (path[0] == '\'' && path[len(path)-1] == '\'')) {
		path = path[1 : len(path)-1]
	}
	if strings.HasPrefix(path, "file:") {
		u, err := url.Parse(path)
		if err != nil || (u.Host != "" && u.Host != "localhost") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
			return "", otp.ErrQRFile
		}
		path = filepath.FromSlash(u.Path)
		if filepath.Separator == '\\' && len(path) > 2 && path[0] == '\\' && path[2] == ':' {
			path = path[1:]
		}
	}
	if path == "" || strings.ContainsAny(path, "\r\n\x00") {
		return "", otp.ErrQRFile
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", otp.ErrQRFile
		}
		path = filepath.Join(home, path[2:])
	}
	return path, nil
}

func (s *StoreForm) importClipboardQR() {
	s.runQRImport(func(ctx context.Context) (string, error) {
		data, err := clipboard.ReadImage(ctx)
		defer clear(data)
		if err != nil {
			return "", err
		}
		return otp.DecodeQR(ctx, data)
	})
}

func (s *StoreForm) runQRImport(decode func(context.Context) (string, error)) {
	if s.qrCancel != nil {
		s.qrCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.qrCancel = cancel
	var value string
	s.run(func(parent context.Context) error {
		work, stop := context.WithCancel(parent)
		defer stop()
		unregister := context.AfterFunc(ctx, stop)
		defer unregister()
		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		value, err = decode(work)
		return err
	}, func(err error) {
		defer cancel()
		if ctx.Err() != nil || !s.qrImport {
			return
		}
		s.qrCancel = nil
		if err != nil {
			s.notify(shared.Error, qrImportError(err))
			return
		}
		s.authenticator = value
		s.qrImport = false
		s.qrPath = ""
		s.notify(shared.Success, "Authenticator imported; save the entry to keep it")
	})
}

func qrImportError(err error) string {
	for _, known := range []error{
		clipboard.ErrImageUnavailable,
		clipboard.ErrImageTimeout,
		clipboard.ErrImageLimit,
		otp.ErrQRFile,
		otp.ErrQRImage,
		otp.ErrQRMissing,
		otp.ErrQRMultiple,
		otp.ErrInvalid,
	} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	return "QR import failed"
}
