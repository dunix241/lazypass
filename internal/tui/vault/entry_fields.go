package vault

import "lazypass/internal/vault"

type entryField struct {
	key, label        string
	secret, multiline bool
}

func (s *StoreForm) entryFields() []entryField {
	switch s.entryType {
	case vault.CardEntry:
		return []entryField{{"cardholderName", "Cardholder", false, false}, {"brand", "Brand", false, false}, {"number", "Number", true, false}, {"expMonth", "Expiry month", false, false}, {"expYear", "Expiry year", false, false}, {"code", "Security code", true, false}, {"notes", "Notes", false, true}}
	case vault.IdentityEntry:
		return []entryField{{"title", "Title", false, false}, {"firstName", "First name", false, false}, {"middleName", "Middle name", false, false}, {"lastName", "Last name", false, false}, {"username", "Username", false, false}, {"company", "Company", false, false}, {"email", "Email", false, false}, {"phone", "Phone", false, false}, {"address1", "Address", false, false}, {"address2", "Address 2", false, false}, {"address3", "Address 3", false, false}, {"city", "City", false, false}, {"state", "State", false, false}, {"postalCode", "Postal code", false, false}, {"country", "Country", false, false}, {"ssn", "SSN", true, false}, {"passportNumber", "Passport", true, false}, {"licenseNumber", "License", true, false}, {"notes", "Notes", false, true}}
	case vault.NoteEntry:
		return []entryField{{"notes", "Note", false, true}}
	case vault.SSHKeyEntry:
		return []entryField{{"privateKey", "Private key", true, true}, {"publicKey", "Public key", false, true}, {"keyFingerprint", "Fingerprint", false, false}, {"notes", "Notes", false, true}}
	default:
		return []entryField{{"password", "Password", true, false}, {"username", "Username", false, false}, {"url", "URL", false, false}, {"notes", "Notes", false, true}}
	}
}

func (s *StoreForm) fieldValue(key string) *string {
	switch key {
	case "password":
		return &s.value
	case "username":
		return &s.username
	case "url":
		return &s.url
	case "notes":
		return &s.notes
	}
	// Map values cannot be addressed; extra fields use a separate editor buffer.
	return &s.extraValue
}
func (s *StoreForm) commitField() {
	fields := s.entryFields()
	if s.field < 0 || s.field >= len(fields) {
		return
	}
	key := fields[s.field].key
	if key != "password" && key != "username" && key != "url" && key != "notes" {
		if s.extraFields == nil {
			s.extraFields = map[string]string{}
		}
		s.extraFields[key] = s.extraValue
	}
}
func (s *StoreForm) selectField(index int) {
	s.commitField()
	s.field = index
	s.extraValue = s.extraFields[s.entryFields()[index].key]
}
func (s *StoreForm) displayField(index int) string {
	f := s.entryFields()[index]
	if index == s.field {
		return *s.fieldValue(f.key)
	}
	switch f.key {
	case "password":
		return s.value
	case "username":
		return s.username
	case "url":
		return s.url
	case "notes":
		return s.notes
	default:
		return s.extraFields[f.key]
	}
}
