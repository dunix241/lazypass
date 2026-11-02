package vault

import (
	"lazypass/internal/vault"
	"testing"
)

func TestPastePreservesMultilineKeyWithoutSubmitting(t *testing.T) {
	s := NewStoreForm(StoreDependencies{})
	s.open, s.editingValue = true, true
	s.entryType = vault.SSHKeyEntry
	s.Paste("-----BEGIN OPENSSH PRIVATE KEY-----\r\nfixture\r\n-----END OPENSSH PRIVATE KEY-----\r\n")
	if s.extraValue != "-----BEGIN OPENSSH PRIVATE KEY-----\nfixture\n-----END OPENSSH PRIVATE KEY-----\n" || !s.open {
		t.Fatal("paste lost key newlines or submitted form")
	}
}
