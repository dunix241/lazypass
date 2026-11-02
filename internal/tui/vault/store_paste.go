package vault

import (
	"strings"
	"unicode"

	"lazypass/internal/tui/shared"
)

func (s *StoreForm) Paste(text string) {
	if !s.open || s.saving || s.loadingEntries || s.choosingType {
		return
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	multiline := s.editingValue && s.entryFields()[s.field].multiline
	text = strings.Map(func(r rune) rune {
		if r == '\n' && multiline {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
	if s.editingValue {
		value := s.activeField()
		if len(*value)+len(text) > 64*1024 {
			s.notify(shared.Warning, "Value limit is 64 KiB")
			return
		}
		*value += text
	} else {
		s.storePath += text
		s.refreshStoreSuggestions()
	}
}
