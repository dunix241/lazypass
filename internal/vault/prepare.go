package vault

import "lazypass/internal/otp"

// PrepareEntry validates changed secrets and derives SSH public key metadata.
func PrepareEntry(entry Entry) (Entry, error) {
	switch entry.Type {
	case LoginEntry:
		if !entry.AuthenticatorChanged {
			return entry, nil
		}
		if entry.AuthenticatorUnavailable {
			return entry, entryValidationError("Authenticator editing is unavailable for this entry")
		}

		label := "Login"
		if len(entry.Path) > 0 {
			label = entry.Path[len(entry.Path)-1]
		}
		value, err := otp.Normalize(entry.Authenticator, label)
		if err != nil {
			return entry, entryValidationError(otp.ErrInvalid.Error())
		}
		entry.Authenticator = value
		return entry, nil

	case SSHKeyEntry:
		return prepareSSHKey(entry)

	default:
		return entry, nil
	}
}
