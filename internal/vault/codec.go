package vault

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"sort"
	"strings"
)

// EncodeEntry produces the conventional pass multiline entry format.
func EncodeEntry(entry Entry) (string, error) {
	if len(entry.Path) == 0 || entry.Path.Validate() != nil {
		return "", ErrInvalidPath
	}
	if entry.Type > LoginEntry {
		if entry.Type > SSHKeyEntry {
			return "", ErrMalformed
		}
		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return "", ErrMalformed
		}
		return "lazypass-entry-v2\n" + string(data) + "\n", nil
	}
	if err := validateEntryValue(entry.Password); err != nil {
		return "", err
	}
	if original := entry.Preserved; original != nil {
		if original.matches(entry) {
			return entry.Password + original.Tail, nil
		}
	}
	if entry.AuthenticatorUnavailable {
		return "", ErrUnsupported
	}
	lines := []string{entry.Password}
	if entry.Authenticator != "" {
		if err := validateEntryValue(entry.Authenticator); err != nil {
			return "", err
		}
		lines = append(lines, entry.Authenticator)
	}
	appendField := func(name, value string) error {
		if err := validateEntryValue(value); err != nil {
			return err
		}
		if value != "" {
			lines = append(lines, name+": "+value)
		}
		return nil
	}
	notesName, notes := "Notes", entry.Notes
	if strings.ContainsAny(notes, "\r\n") {
		notesName = "Lazypass-Notes-Base64"
		notes = base64.StdEncoding.EncodeToString([]byte(notes))
	}
	for _, field := range []struct{ name, value string }{
		{"Username", entry.Username},
		{"URL", entry.URL},
		{notesName, notes},
	} {
		if err := appendField(field.name, field.value); err != nil {
			return "", err
		}
	}
	keys := make([]string, 0, len(entry.Fields))
	for key := range entry.Fields {
		if key == "Lazypass-Notes-Base64" {
			return "", fmt.Errorf("reserved entry field name")
		}
		if key == "" || strings.ContainsAny(key, "\r\n:") {
			return "", fmt.Errorf("invalid entry field name")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := appendField(key, entry.Fields[key]); err != nil {
			return "", err
		}
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func (original *EntryText) matches(entry Entry) bool {
	return entry.Authenticator == original.Authenticator &&
		entry.Username == original.Username &&
		entry.URL == original.URL &&
		entry.Notes == original.Notes &&
		maps.Equal(entry.Fields, original.Fields)
}

func validateEntryValue(value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("vault entry values must be single-line")
	}
	return nil
}

// DecodeEntry decodes a pass show result without placing its contents in errors.
func DecodeEntry(path Path, text string) (Entry, error) {
	if len(path) == 0 || path.Validate() != nil {
		return Entry{}, ErrInvalidPath
	}
	if strings.HasPrefix(text, "lazypass-entry-v2\n") {
		return decodeTypedEntry(path, []byte(strings.TrimPrefix(text, "lazypass-entry-v2\n")))
	}
	if strings.HasPrefix(text, "lazypass-entry-v1:") {
		data, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(strings.TrimPrefix(text, "lazypass-entry-v1:"), "\n"))
		if err != nil {
			return Entry{}, ErrMalformed
		}
		return decodeTypedEntry(path, data)
	}
	original := text
	text = strings.TrimSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	entry := Entry{Path: append(Path(nil), path...), Password: strings.TrimSuffix(lines[0], "\r"), Fields: map[string]string{}}
	var freeform []string
	authenticatorCount := 0
	for _, line := range lines[1:] {
		if strings.HasPrefix(strings.TrimSuffix(line, "\r"), "otpauth://") {
			authenticatorCount++
			entry.Authenticator = strings.TrimSuffix(line, "\r")
			continue
		}
		key, value, ok := strings.Cut(strings.TrimSuffix(line, "\r"), ": ")
		if !ok || key == "" {
			freeform = append(freeform, line)
			continue
		}
		switch key {
		case "Username":
			entry.Username = value
		case "URL":
			entry.URL = value
		case "Notes":
			entry.Notes = value
		case "Lazypass-Notes-Base64":
			decoded, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				freeform = append(freeform, line)
				continue
			}
			entry.Notes = string(decoded)
		default:
			entry.Fields[key] = value
		}
	}
	if authenticatorCount > 1 {
		entry.Authenticator = ""
		entry.AuthenticatorUnavailable = true
	}
	if strings.HasPrefix(entry.Password, "otpauth://") {
		entry.AuthenticatorUnavailable = true
	}
	if len(freeform) > 0 {
		if entry.Notes != "" {
			entry.Notes += "\n"
		}
		entry.Notes += strings.Join(freeform, "\n")
	}
	tail := ""
	if index := strings.IndexByte(original, '\n'); index >= 0 {
		start := index
		if index > 0 && original[index-1] == '\r' {
			start--
		}
		tail = original[start:]
	}
	entry.Preserved = &EntryText{
		Tail:          tail,
		Authenticator: entry.Authenticator,
		Username:      entry.Username,
		URL:           entry.URL,
		Notes:         entry.Notes,
		Fields:        maps.Clone(entry.Fields),
	}
	return entry, nil
}

func decodeTypedEntry(path Path, data []byte) (Entry, error) {
	var entry Entry
	if json.Unmarshal(data, &entry) != nil || entry.Type == LoginEntry || entry.Type > SSHKeyEntry {
		return Entry{}, ErrMalformed
	}
	entry.Path = append(Path(nil), path...)
	return entry, nil
}
