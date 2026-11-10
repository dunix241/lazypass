package vault

import (
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
	return input.Options{Multiline: field.multiline, Secret: field.secret, MaxBytes: fieldByteLimit}
}

func (s *StoreForm) activeInput() (*string, *input.Editor, input.Options) {
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
		s.notify(shared.Warning, "Value limit is 64 KiB")
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
