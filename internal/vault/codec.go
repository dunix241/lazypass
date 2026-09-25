package vault

import (
	"fmt"
	"sort"
	"strings"
)

// EncodeEntry produces the conventional pass multiline entry format.
func EncodeEntry(entry Entry) (string, error) {
	if err := entry.Path.Validate(); err != nil || len(entry.Path) == 0 {
		if err == nil {
			err = ErrInvalidPath
		}
		return "", err
	}
	if err := validateEntryValue(entry.Password); err != nil {
		return "", err
	}
	lines := []string{entry.Password}
	appendField := func(name, value string) error {
		if err := validateEntryValue(value); err != nil {
			return err
		}
		if value != "" {
			lines = append(lines, name+": "+value)
		}
		return nil
	}
	for _, field := range []struct{ name, value string }{{"Username", entry.Username}, {"URL", entry.URL}, {"Notes", entry.Notes}} {
		if err := appendField(field.name, field.value); err != nil {
			return "", err
		}
	}
	keys := make([]string, 0, len(entry.Fields))
	for key := range entry.Fields {
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

func validateEntryValue(value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("vault entry values must be single-line")
	}
	return nil
}

// DecodeEntry decodes a pass show result without placing its contents in errors.
func DecodeEntry(path Path, text string) (Entry, error) {
	if err := path.Validate(); err != nil || len(path) == 0 {
		if err == nil {
			err = ErrInvalidPath
		}
		return Entry{}, err
	}
	text = strings.TrimSuffix(text, "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return Entry{}, fmt.Errorf("malformed vault entry")
	}
	entry := Entry{Path: append(Path(nil), path...), Password: strings.TrimSuffix(lines[0], "\r"), Fields: map[string]string{}}
	for _, line := range lines[1:] {
		key, value, ok := strings.Cut(strings.TrimSuffix(line, "\r"), ": ")
		if !ok || key == "" {
			return Entry{}, fmt.Errorf("malformed vault entry")
		}
		switch key {
		case "Username":
			entry.Username = value
		case "URL":
			entry.URL = value
		case "Notes":
			entry.Notes = value
		default:
			entry.Fields[key] = value
		}
	}
	return entry, nil
}
