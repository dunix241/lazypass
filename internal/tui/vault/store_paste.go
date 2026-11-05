package vault

func (s *StoreForm) Paste(text string) {
	if !s.open || s.saving || s.loadingEntries || s.choosingType {
		return
	}
	value, editor, options := s.activeInput()
	s.applyEditor(value, editor, editor.Insert(text, options))
}
