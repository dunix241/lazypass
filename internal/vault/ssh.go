package vault

import (
	"encoding/base64"
	"encoding/pem"
	"errors"
	"maps"
	"strings"

	"golang.org/x/crypto/ssh"
)

type entryValidationError string

func (e entryValidationError) Error() string       { return string(e) }
func (e entryValidationError) UserMessage() string { return string(e) }
func (e entryValidationError) Unwrap() error       { return ErrMalformed }

// ValidateSSHKey returns only fixed messages, never parser errors containing input.
func ValidateSSHKey(entry Entry) error {
	_, err := PrepareEntry(entry)
	return err
}

// PrepareEntry validates SSH fields and derives the public key and fingerprint.
func PrepareEntry(entry Entry) (Entry, error) {
	if entry.Type != SSHKeyEntry {
		return entry, nil
	}
	private := normalizePrivateKey(entry.Fields["privateKey"])
	if private == "" {
		return entry, entryValidationError("Enter an SSH private key")
	}
	key, err := ssh.ParseRawPrivateKey([]byte(private))
	if err != nil {
		var encrypted *ssh.PassphraseMissingError
		if errors.As(err, &encrypted) {
			return entry, entryValidationError("Passphrase-encrypted SSH keys are not supported by this editor")
		}
		return entry, entryValidationError("Invalid private key: paste the full key block")
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return entry, entryValidationError("Unsupported SSH private key type")
	}
	if public := strings.TrimSpace(entry.Fields["publicKey"]); public != "" {
		parsed, _, _, rest, err := ssh.ParseAuthorizedKey([]byte(public))
		if err != nil || len(strings.TrimSpace(string(rest))) != 0 {
			return entry, entryValidationError("Invalid SSH public key: use an authorized_keys line")
		}
		if string(parsed.Marshal()) != string(signer.PublicKey().Marshal()) {
			return entry, entryValidationError("SSH public key does not match the private key")
		}
	}
	entry.Fields = maps.Clone(entry.Fields)
	entry.Fields["privateKey"] = private + "\n"
	if strings.TrimSpace(entry.Fields["publicKey"]) == "" {
		entry.Fields["publicKey"] = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	}
	entry.Fields["keyFingerprint"] = ssh.FingerprintSHA256(signer.PublicKey())
	return entry, nil
}

func normalizePrivateKey(value string) string {
	value = strings.TrimSpace(value)
	if block, rest := pem.Decode([]byte(value)); block != nil && len(rest) == 0 {
		return value
	}
	for _, kind := range []string{"OPENSSH PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY", "PRIVATE KEY", "ENCRYPTED PRIVATE KEY"} {
		header, footer := "-----BEGIN "+kind+"-----", "-----END "+kind+"-----"
		if !strings.HasPrefix(value, header) || !strings.HasSuffix(value, footer) {
			continue
		}
		body := strings.TrimSuffix(strings.TrimPrefix(value, header), footer)
		body = strings.Join(strings.Fields(body), "")
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			return value
		}
		return string(pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: decoded}))
	}
	return value
}
