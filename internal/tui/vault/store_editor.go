package vault

import (
	"fmt"
	"lazypass/internal/otp"
	"lazypass/internal/tui/input"
	"lazypass/internal/tui/shared"

	"github.com/gdamore/tcell/v2"
)

const fieldByteLimit = 64 * 1024

func (s *StoreForm) selectedLocation() bool {
	if len(s.location) == 0 {
		return false
	}
	if s.location[0] == "Folders" {
		return len(s.location) >= 2
	}
	return len(s.location) >= 3
}

func (s *StoreForm) editor(key, value string) *input.Editor {
	if s.editors == nil {
		s.editors = make(map[string]*input.Editor)
	}
	if s.editors[key] == nil {
		s.editors[key] = &input.Editor{}
	}
	editor := s.editors[key]
	editor.Sync(value)
	return editor
}

func (field entryField) inputOptions() input.Options {
	limit := fieldByteLimit
	if field.key == "authenticator" {
		limit = otp.MaxInput
	}
	return input.Options{Multiline: field.multiline, Secret: field.secret, MaxBytes: limit}
}

func (s *StoreForm) activeInput() (*string, *input.Editor, input.Options) {
	if s.qrImport || s.qrExport {
		return &s.qrPath, s.editor("qr-path", s.qrPath), input.Options{MaxBytes: fieldByteLimit}
	}
	if !s.editingValue {
		if s.locationOnly {
			return &s.entryName, s.editor("entry-name", s.entryName), input.Options{MaxBytes: fieldByteLimit}
		}
		return &s.storePath, s.editor("destination", s.storePath), input.Options{MaxBytes: fieldByteLimit, WordSeparators: "/"}
	}
	field := s.entryFields()[s.field]
	value := s.fieldValue(field.key)
	return value, s.editor(field.key, *value), field.inputOptions()
}

func (s *StoreForm) editInput(event *tcell.EventKey) {
	value, editor, options := s.activeInput()
	_, err := editor.HandleKey(event, options)
	s.applyEditor(value, editor, err)
}

func (s *StoreForm) applyEditor(value *string, editor *input.Editor, err error) {
	if err != nil {
		_, _, options := s.activeInput()
		s.notify(shared.Warning, fmt.Sprintf("Value limit is %d KiB", options.MaxBytes/1024))
		return
	}
	changed := *value != editor.Value()
	*value = editor.Value()
	if changed && !s.editingValue {
		if s.locationOnly {
			s.storePath = s.location.String() + "/" + s.entryName
			return
		}
		s.refreshStoreSuggestions()
	}
}
