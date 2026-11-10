package vault

import "lazypass/internal/vault"

type entryField struct {
	key, label        string
	secret, multiline bool
}

var loginFields = []entryField{
	{"password", "Password", true, false},
	{"username", "Username", false, false},
	{"url", "URL", false, false},
	{"notes", "Notes", false, true},
	{"authenticator", "Authenticator", true, false},
}

var cardFields = []entryField{
	{"cardholderName", "Cardholder", false, false},
	{"brand", "Brand", false, false},
	{"number", "Number", true, false},
	{"expMonth", "Expiry month", false, false},
	{"expYear", "Expiry year", false, false},
	{"code", "Security code", true, false},
	{"notes", "Notes", false, true},
}

var identityFields = []entryField{
	{"title", "Title", false, false},
	{"firstName", "First name", false, false},
	{"middleName", "Middle name", false, false},
	{"lastName", "Last name", false, false},
	{"username", "Username", false, false},
	{"company", "Company", false, false},
	{"email", "Email", false, false},
	{"phone", "Phone", false, false},
	{"address1", "Address", false, false},
	{"address2", "Address 2", false, false},
	{"address3", "Address 3", false, false},
	{"city", "City", false, false},
	{"state", "State", false, false},
	{"postalCode", "Postal code", false, false},
	{"country", "Country", false, false},
	{"ssn", "SSN", true, false},
	{"passportNumber", "Passport", true, false},
	{"licenseNumber", "License", true, false},
	{"notes", "Notes", false, true},
}

var noteFields = []entryField{
	{"notes", "Note", false, true},
}

var sshFields = []entryField{
	{"privateKey", "Private key", true, true},
	{"publicKey", "Public key", false, true},
	{"keyFingerprint", "Fingerprint", false, false},
	{"notes", "Notes", false, true},
}

func (s *StoreForm) entryFields() []entryField {
	switch s.entryType {
	case vault.CardEntry:
		return cardFields
	case vault.IdentityEntry:
		return identityFields
	case vault.NoteEntry:
		return noteFields
	case vault.SSHKeyEntry:
		return sshFields
	default:
		if s.editEntry != nil && s.editEntry.AuthenticatorUnavailable {
			return loginFields[:len(loginFields)-1]
		}
		return loginFields
	}
}

func (s *StoreForm) directField(key string) *string {
	switch key {
	case "password":
		return &s.value
	case "username":
		return &s.username
	case "url":
		return &s.url
	case "notes":
		return &s.notes
	case "authenticator":
		return &s.authenticator
	}
	return nil
}

func (s *StoreForm) fieldValue(key string) *string {
	if value := s.directField(key); value != nil {
		return value
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
	if s.directField(key) == nil {
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
	if value := s.directField(f.key); value != nil {
		return *value
	}
	return s.extraFields[f.key]
}
