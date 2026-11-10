package vault

import (
	"strings"
	"testing"
)

func TestAuthenticatorCodecReplacementRemovalAndPreservation(t *testing.T) {
	const uri = "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP"
	original := "password\r\nUsername: user\r\n" + uri + "\r\nexternal notes\r\n"
	entry, err := DecodeEntry(Path{"test"}, original)
	if err != nil || entry.Authenticator != uri || strings.Contains(entry.Notes, "otpauth") {
		t.Fatal("authenticator was not separated from notes")
	}
	entry.Password = "new"
	encoded, err := EncodeEntry(entry)
	if err != nil || encoded != "new"+strings.TrimPrefix(original, "password") {
		t.Fatal("password edit did not preserve original tail")
	}
	entry.Authenticator = ""
	entry.AuthenticatorChanged = true
	encoded, err = EncodeEntry(entry)
	if err != nil || strings.Contains(encoded, "otpauth") {
		t.Fatal("authenticator removal retained seed")
	}
	decoded, err := DecodeEntry(Path{"test"}, encoded)
	if err != nil || decoded.Notes != entry.Notes {
		t.Fatal("notes changed during removal")
	}
}

func TestAmbiguousAuthenticatorDoesNotRewriteExternalText(t *testing.T) {
	original := "password\notpauth://totp/One?secret=AAA\notpauth://totp/Two?secret=BBB\n"
	entry, _ := DecodeEntry(Path{"test"}, original)
	if !entry.AuthenticatorUnavailable {
		t.Fatal("ambiguous authenticators were accepted")
	}
	entry.Password = "updated"
	encoded, err := EncodeEntry(entry)
	if err != nil || encoded != strings.Replace(original, "password", "updated", 1) {
		t.Fatal("external tail changed")
	}
	entry.Username = "changed"
	if _, err := EncodeEntry(entry); err != ErrUnsupported {
		t.Fatal("ambiguous content was rewritten")
	}
}

func TestAuthenticatorOnlyLoginKeepsEmptyPasswordLine(t *testing.T) {
	const uri = "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP"
	entry := Entry{Path: Path{"entry"}, Authenticator: uri}
	text, err := EncodeEntry(entry)
	if err != nil || text != "\n"+uri+"\n" {
		t.Fatal("empty password line was not retained")
	}
	decoded, err := DecodeEntry(entry.Path, text)
	if err != nil || decoded.Password != "" || decoded.Username != "" || decoded.Authenticator != uri || decoded.AuthenticatorUnavailable {
		t.Fatal("authenticator-only Login did not round trip")
	}
}
