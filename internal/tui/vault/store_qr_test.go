package vault

import (
	"context"
	"image/png"
	"lazypass/internal/otp"
	"os"
	"path/filepath"
	"testing"

	"lazypass/internal/vault/service"

	"github.com/gdamore/tcell/v2"
	libotp "github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func TestQRImportPopulatesMaskedFieldWithoutSaving(t *testing.T) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "PublicFixture", AccountName: "fixture", Secret: []byte("12345678901234567890"), Digits: libotp.DigitsSix})
	if err != nil {
		t.Fatal(err)
	}
	img, err := key.Image(300, 300)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fixture.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = png.Encode(file, img)
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	p := &memoryVault{}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	s.OpenAdd(nil)
	s.choosingType = false
	s.editingValue = true
	s.selectField(len(s.entryFields()) - 1)
	s.HandleKey(tcell.NewEventKey(tcell.KeyF2, 0, 0))
	s.Paste(path)
	s.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if s.qrImport || s.authenticator != key.URL() || p.store.Authenticator != "" || !s.entryFields()[s.field].secret {
		t.Fatal("QR import did not populate masked unsaved field")
	}
}

func TestQRImagePathNormalization(t *testing.T) {
	for _, input := range []string{`"/tmp/QR image.png"`, `'/tmp/QR image.png'`, "file:///tmp/QR%20image.png", "file://localhost/tmp/QR%20image.png"} {
		path, err := qrImagePath(input)
		if err != nil || path != filepath.FromSlash("/tmp/QR image.png") {
			t.Fatal("pasted path normalization failed")
		}
	}
	for _, input := range []string{"file://remote/private.png", "file:///tmp/image.png?x=1", "\x00", "first\nsecond"} {
		if _, err := qrImagePath(input); err != otp.ErrQRFile {
			t.Fatal("invalid path accepted")
		}
	}
}

func TestEmptyPasteRequestsClipboardImageOnlyInQRPrompt(t *testing.T) {
	calls := 0
	s := NewStoreForm(StoreDependencies{RunOperation: func(func(context.Context) error, func(error)) { calls++ }})
	s.open = true
	s.Paste("")
	if calls != 0 {
		t.Fatal("ordinary empty paste read clipboard")
	}
	s.qrImport = true
	s.Paste("")
	if calls != 1 {
		t.Fatal("empty QR paste did not request image")
	}
	s.Close()
}

func TestQRImportCloseRejectsLateResult(t *testing.T) {
	var work func(context.Context) error
	var done func(error)
	s := NewStoreForm(StoreDependencies{RunOperation: func(w func(context.Context) error, d func(error)) { work, done = w, d }})
	s.open = true
	s.qrImport = true
	s.authenticator = "existing"
	s.qrPath = "missing"
	s.importQR()
	s.Close()
	err := work(context.Background())
	done(err)
	if err != context.Canceled || s.IsOpen() || s.authenticator != "" {
		t.Fatal("closed form accepted late QR result")
	}
}
