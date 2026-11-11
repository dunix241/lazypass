package vault

import "github.com/gdamore/tcell/v2"

func (s *StoreForm) Paste(text string) {
	if !s.open || s.saving || s.loadingEntries {
		return
	}
	if s.choosingType {
		for _, r := range text {
			s.HandleKey(tcell.NewEventKey(tcell.KeyRune, r, 0))
		}
		return
	}
	if s.bulk != nil {
		if s.bulk.phase == "images" && text == "" {
			s.loadBulkImage(true)
			return
		}
		if s.bulk.phase != "images" && s.bulk.phase != "destination" && s.bulk.phase != "rename" {
			return
		}
	}
	if s.qrImage != nil && !s.qrExport {
		return
	}
	if s.qrImport && text == "" {
		s.importClipboardQR()
		return
	}
	value, editor, options := s.activeInput()
	s.applyEditor(value, editor, editor.Insert(text, options))
}
