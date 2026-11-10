package vault

func (s *StoreForm) Paste(text string) {
	if !s.open || s.saving || s.loadingEntries || s.choosingType {
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
