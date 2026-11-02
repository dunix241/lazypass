package vault

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestSSHKeyValidation(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(private, "")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	entry := Entry{Type: SSHKeyEntry, Fields: map[string]string{"privateKey": string(pem.EncodeToMemory(block)), "publicKey": string(ssh.MarshalAuthorizedKey(pub))}}
	if err := ValidateSSHKey(entry); err != nil {
		t.Fatal(err)
	}
	flattened := Entry{Type: SSHKeyEntry, Fields: map[string]string{"privateKey": strings.ReplaceAll(entry.Fields["privateKey"], "\n", "")}}
	if _, err := PrepareEntry(flattened); err != nil {
		t.Fatalf("single-line pasted key not normalized: %v", err)
	}
	minimal := Entry{Type: SSHKeyEntry, Fields: map[string]string{"privateKey": entry.Fields["privateKey"]}}
	prepared, err := PrepareEntry(minimal)
	if err != nil || prepared.Fields["publicKey"] != strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub))) || prepared.Fields["keyFingerprint"] != ssh.FingerprintSHA256(pub) {
		t.Fatal("SSH metadata was not derived")
	}
	if minimal.Fields["publicKey"] != "" || minimal.Fields["keyFingerprint"] != "" {
		t.Fatal("preparation mutated caller fields")
	}
	entry.Fields["publicKey"] = "not-a-public-key"
	if err := ValidateSSHKey(entry); err == nil {
		t.Fatal("invalid public key accepted")
	}
	entry.Fields["privateKey"] = "private-fixture-that-must-not-leak"
	err = ValidateSSHKey(entry)
	if err == nil || strings.Contains(err.Error(), entry.Fields["privateKey"]) {
		t.Fatal("invalid private key accepted or exposed")
	}
}
