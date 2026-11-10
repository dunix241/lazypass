package vault

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazypass/internal/otp"
	"lazypass/internal/vault"
	"lazypass/internal/vault/service"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/gdamore/tcell/v2"
)

func TestBulkImagePreviewAndCreate(t *testing.T) {
	data, err := hex.DecodeString("0a2a0a1431323334353637383930313233343536373839301205616c6963651a074578616d706c65200128013002100118012000282a")
	if err != nil {
		t.Fatal(err)
	}
	uri := "otpauth-migration://offline?data=" + url.QueryEscape(base64.StdEncoding.EncodeToString(data))
	code, err := qr.Encode(uri, qr.M, qr.Auto)
	if err != nil {
		t.Fatal(err)
	}
	scaled, err := barcode.Scale(code, 500, 500)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 580, 580))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(40, 40, 540, 540), scaled, image.Point{}, draw.Src)
	path := filepath.Join(t.TempDir(), "migration.png")
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
	s.open = true
	s.bulk = &bulkImport{phase: "images", text: path}
	s.loadBulkImage(false)
	if s.bulk.phase != "preview" || len(s.bulk.rows) != 1 || !s.bulk.rows[0].selected {
		t.Fatal("migration image did not reach preview")
	}
	s.handleBulkKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if s.bulk.phase != "destination" {
		t.Fatal("destination missing")
	}
	s.bulk.text = "imported"
	s.executeBulk()
	if s.bulk.phase != "results" || !s.bulk.changed || p.store.Path.String() != "imported/Example - alice" || p.store.Password != "" || p.store.Authenticator == "" {
		t.Fatal("authenticator batch create failed")
	}
}

func TestBulkStepperShowsOutOfOrderPageProgress(t *testing.T) {
	data, err := hex.DecodeString("0a2a0a1431323334353637383930313233343536373839301205616c6963651a074578616d706c65200128013002100218022001282a")
	if err != nil {
		t.Fatal(err)
	}
	page, err := otp.ParseMigration("otpauth-migration://offline?data=" + url.QueryEscape(base64.StdEncoding.EncodeToString(data)))
	if err != nil {
		t.Fatal(err)
	}
	s := NewStoreForm(StoreDependencies{})
	s.bulk = &bulkImport{phase: "images"}
	if err := s.bulk.batch.Add(page); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	frame := testFrame()
	s.drawBulk(screen, frame)
	var text strings.Builder
	for y := 0; y < 24; y++ {
		for x := 0; x < 80; x++ {
			cell, _, _ := screen.Get(x, y)
			text.WriteString(cell)
		}
		text.WriteByte('\n')
	}
	for _, want := range []string{"Step 1 of 4", "1 of 2 export pages loaded", "Export page 1 needed", "Export page 2 loaded"} {
		if !strings.Contains(text.String(), want) {
			t.Fatalf("missing progress text %q", want)
		}
	}
}

func TestBulkConflictDoesNotOverwrite(t *testing.T) {
	p := &memoryVault{nodes: map[string][]vault.Node{"target": {{Name: "existing", Kind: vault.EntryNode}}}}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	s.open = true
	s.bulk = &bulkImport{phase: "destination", text: "target", rows: []bulkRow{{name: "existing", selected: true, record: otp.MigrationRecord{URI: "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP"}}}}
	s.executeBulk()
	if p.store.Authenticator != "" || s.bulk.changed || s.bulk.rows[0].status == "" {
		t.Fatal("conflict was overwritten")
	}
}

func TestBulkRenameEscapePreservesImportProgress(t *testing.T) {
	s := NewStoreForm(StoreDependencies{})
	s.open = true
	s.bulk = &bulkImport{
		phase: "preview",
		rows:  []bulkRow{{name: "Original account", selected: true}},
	}
	s.handleBulkKey(tcell.NewEventKey(tcell.KeyRune, 'r', 0))
	s.bulk.text = "Unsaved rename"
	s.handleBulkKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
	if !s.IsOpen() || s.bulk == nil || s.bulk.phase != "preview" ||
		s.bulk.rows[0].name != "Original account" || !s.bulk.rows[0].selected || s.bulk.text != "" {
		t.Fatal("canceling rename lost import progress or changed the account")
	}
}

func TestBulkDestinationBackPreservesReviewAndBlocksDuplicateNames(t *testing.T) {
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: &memoryVault{}}})
	s.open = true
	s.bulk = &bulkImport{phase: "destination", destination: vault.Path{"target"}, rows: []bulkRow{
		{name: "same", selected: true},
		{name: "same", selected: true},
	}}
	s.checkBulkConflicts()
	if s.bulk.phase != "preview" || s.bulk.rows[1].status == "" {
		t.Fatal("duplicate names reached execution")
	}
	s.bulk.phase = "destination"
	s.handleBulkKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
	if !s.open || s.bulk.phase != "preview" || !s.bulk.rows[0].selected {
		t.Fatal("destination back lost progress")
	}
}

func TestBulkRetryExcludesConfirmedAndUncertainRows(t *testing.T) {
	p := &memoryVault{}
	s := NewStoreForm(StoreDependencies{Service: service.Service{Provider: p}})
	s.open = true
	s.bulk = &bulkImport{phase: "destination", text: "target", rows: []bulkRow{
		{name: "done", selected: true, created: true},
		{name: "uncertain", selected: true, uncertain: true},
		{name: "retry", selected: true, record: otp.MigrationRecord{URI: "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP"}},
	}}
	s.executeBulk()
	if p.store.Path.String() != "target/retry" || s.bulk.rows[0].status != "" || s.bulk.rows[1].status != "" {
		t.Fatal("unsafe rows retried")
	}
}

func TestBulkEligibilityAndNameValidation(t *testing.T) {
	for _, row := range []bulkRow{
		{selected: true, created: true},
		{selected: true, uncertain: true},
		{selected: true, record: otp.MigrationRecord{Unsupported: "Unsupported OTP type"}},
		{},
	} {
		if row.importable() {
			t.Fatal("ineligible row can be imported")
		}
	}
	for _, name := range []string{"", "nested/name", "nested\\name", ".."} {
		if _, err := (bulkRow{name: name}).destinationPath(vault.Path{"target"}); err == nil {
			t.Fatal("invalid entry name accepted")
		}
	}
	path, err := (bulkRow{name: "Account 😀"}).destinationPath(vault.Path{"target"})
	if err != nil || path.String() != "target/Account 😀" {
		t.Fatal("valid entry name changed")
	}
}

func TestBulkCloseRejectsLateImage(t *testing.T) {
	var work func(context.Context) error
	var done func(error)
	s := NewStoreForm(StoreDependencies{RunOperation: func(w func(context.Context) error, d func(error)) { work, done = w, d }})
	s.open = true
	s.bulk = &bulkImport{phase: "images", text: "missing.png"}
	s.loadBulkImage(false)
	s.Close()
	done(work(context.Background()))
	if s.bulk != nil || s.IsOpen() {
		t.Fatal("late image restored closed session")
	}
}
