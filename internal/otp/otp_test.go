package otp

import (
	"encoding/base32"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRFC6238AndRollover(t *testing.T) {
	for _, tc := range []struct {
		algorithm, secret, want string
	}{
		{"SHA1", "12345678901234567890", "94287082"},
		{"SHA256", "12345678901234567890123456789012", "46119246"},
		{"SHA512", "1234567890123456789012345678901234567890123456789012345678901234", "90693936"},
	} {
		secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(tc.secret))
		key, err := Parse(fmt.Sprintf("otpauth://totp/Test?secret=%s&algorithm=%s&digits=8", secret, tc.algorithm))
		if err != nil {
			t.Fatal(err)
		}
		code, expiry, err := key.Code(time.Unix(59, 999999999))
		if err != nil || code != tc.want || !expiry.Equal(time.Unix(60, 0)) {
			t.Fatalf("RFC vector failed for %s", tc.algorithm)
		}
		_, expiry, err = key.Code(time.Unix(60, 0))
		if err != nil || !expiry.Equal(time.Unix(90, 0)) {
			t.Fatal("incorrect rollover")
		}
	}
}

func TestRejectMalformedWithoutSecretErrors(t *testing.T) {
	const prefix = "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP"
	for _, value := range []string{
		"", "!invalid!", strings.Repeat("A", MaxInput+1), prefix + "&secret=OTHER",
		prefix + "&algorithm=MD5", prefix + "&digits=7", prefix + "&period=0",
		prefix + "&period=86401", prefix + "&period=-1", prefix + "&period=99999999999999999999",
		prefix + "&period=no", prefix + "&encoder=steam", prefix + "&counter=1",
		prefix + "#fragment", prefix + "&issuer=%zz", prefix + "&issuer=%0a",
		prefix + "#", "otpauth://totp/Issuer:?secret=JBSWY3DPEHPK3PXP",
		"otpauth://totp/:account?secret=JBSWY3DPEHPK3PXP",
		"otpauth://totp/Issuer:account:extra?secret=JBSWY3DPEHPK3PXP",
		"otpauth://totp/Issuer:account?secret=JBSWY3DPEHPK3PXP&issuer=Other",
		"otpauth://hotp/Test?secret=JBSWY3DPEHPK3PXP", "https://example.invalid",
	} {
		_, err := Parse(value)
		if err != ErrInvalid {
			t.Fatal("expected static validation error")
		}
	}
}

func TestNormalizePlainKeyAndPreserveURI(t *testing.T) {
	value, err := Normalize("jbswy3dpehpk3pxp", "Account 😀")
	if err != nil || value != "otpauth://totp/Account%20%F0%9F%98%80?secret=JBSWY3DPEHPK3PXP" {
		t.Fatal("incorrect URI encoding")
	}
	uri := "otpauth://totp/Example:account?secret=JBSWY3DPEHPK3PXP&issuer=Example&period=60"
	value, err = Normalize(uri, "Other")
	if err != nil || value != uri {
		t.Fatal("external URI changed")
	}
}
