package vault

import (
	"context"
	"image"
	"image/color"
	"path/filepath"
	"strings"

	"lazypass/internal/otp"
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

func (s *StoreForm) showQR() {
	value := s.authenticator
	label := "Login"
	if name := filepath.Base(strings.TrimRight(s.storePath, "/")); name != "." && name != "" {
		label = name
	}
	var img image.Image
	s.run(func(ctx context.Context) error {
		var err error
		img, err = otp.QRImage(ctx, value, label)
		return err
	}, func(err error) {
		if err != nil {
			s.notify(shared.Error, "Cannot show QR: enter a supported authenticator key")
			return
		}
		s.qrImage = img
	})
}

func (s *StoreForm) handleQRDisplayKey(event *tcell.EventKey) {
	if s.saving {
		return
	}
	if s.qrExport {
		switch event.Key() {
		case tcell.KeyEscape:
			s.qrExport = false
			s.qrPath = ""
		case tcell.KeyEnter:
			s.exportQR()
		default:
			s.editInput(event)
		}
		return
	}
	switch event.Key() {
	case tcell.KeyEscape:
		s.qrImage = nil
	case tcell.KeyRune:
		if event.Rune() == 's' {
			s.qrExport = true
			s.qrPath = ""
		}
	}
}

func (s *StoreForm) exportQR() {
	path, err := qrImagePath(s.qrPath)
	if err != nil {
		s.notify(shared.Error, "Choose a new PNG file path")
		return
	}
	img := s.qrImage
	s.run(func(ctx context.Context) error { return otp.ExportQR(ctx, path, img) }, func(err error) {
		if err != nil {
			s.notify(shared.Error, otp.ErrQRExport.Error())
			return
		}
		s.qrExport = false
		s.qrPath = ""
		s.notify(shared.Success, "QR image exported")
	})
}

func (s *StoreForm) drawQRDisplay(screen tcell.Screen, frame shared.Frame) {
	b := s.qrImage.Bounds()
	w, h := b.Dx()+4, (b.Dy()+1)/2+4
	r := s.qrDisplayRect(frame.Viewport)
	shared.Panel{Bounds: r, Title: "Authenticator QR", TitleColor: frame.Palette.Accent, Border: frame.Palette.Focus, Background: frame.Palette.Surface}.Draw(screen, nil)
	if s.qrExport {
		shared.PrintAt(screen, r.X+2, r.Y+2, "Export PNG to a new file:", frame.Palette.Text)
		_, editor, options := s.activeInput()
		editor.Draw(screen, r.X+2, r.Y+3, r.W-4, 1, tcell.StyleDefault.Foreground(frame.Palette.Text).Background(frame.Palette.Surface), !s.saving, options)
		drawPopupActions(screen, r, s.qrActions(), frame.Palette.Muted)
		return
	}
	if r.W < w || r.H < h {
		for i, line := range shared.WrapText("Terminal too small; export PNG instead", r.W-4) {
			if i >= r.H-4 {
				break
			}
			shared.PrintAt(screen, r.X+2, r.Y+2+i, line, frame.Palette.Muted)
		}
	} else {
		for y := 0; y < b.Dy(); y += 2 {
			for x := 0; x < b.Dx(); x++ {
				top, bottom := qrModuleColor(s.qrImage, b.Min.X+x, b.Min.Y+y), tcell.ColorWhite
				if y+1 < b.Dy() {
					bottom = qrModuleColor(s.qrImage, b.Min.X+x, b.Min.Y+y+1)
				}
				screen.SetContent(r.X+2+x, r.Y+2+y/2, '▀', nil, tcell.StyleDefault.Foreground(top).Background(bottom))
			}
		}
	}
	drawPopupActions(screen, r, s.qrActions(), frame.Palette.Muted)
}

func (s *StoreForm) qrDisplayRect(viewport shared.Rect) shared.Rect {
	b := s.qrImage.Bounds()
	return shared.CenteredPanel(viewport, min(b.Dx()+4, viewport.W-2), min((b.Dy()+1)/2+4, viewport.H-2))
}

func qrModuleColor(img image.Image, x, y int) tcell.Color {
	if color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y < 128 {
		return tcell.ColorBlack
	}
	return tcell.ColorWhite
}
