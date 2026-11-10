package service

import (
	"context"
	"testing"
	"time"

	"lazypass/internal/vault"
)

func TestAuthenticatorSessionRolloverAndExpiry(t *testing.T) {
	s := Service{Provider: typedCopyProvider{entry: vault.Entry{Authenticator: "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP&period=37"}}}
	value, err := s.LoadAuthenticator(context.Background(), vault.Path{"test"})
	if err != nil || value.key == nil || value.Expiry.Unix()%37 != 0 {
		t.Fatal("local session timing invalid")
	}
	if code, seconds := value.Current(value.Expiry); code != "" || seconds != 0 {
		t.Fatal("expired code remained visible")
	}
	if code, seconds := value.Current(value.Expiry.Add(-time.Millisecond)); len(code) != 6 || seconds != 1 {
		t.Fatal("fractional remaining validity invalid")
	}
}

type nativeSessionProvider struct{ typedCopyProvider }

func (nativeSessionProvider) AuthenticatorCode(context.Context, vault.Path) (string, error) {
	return "123456", nil
}

func TestNativeSessionRetainsTimingWithoutSeed(t *testing.T) {
	s := Service{Provider: nativeSessionProvider{typedCopyProvider{entry: vault.Entry{Authenticator: "otpauth://totp/Test?secret=JBSWY3DPEHPK3PXP&period=37"}}}}
	value, err := s.LoadAuthenticator(context.Background(), vault.Path{"test"})
	if err != nil || value.key != nil || value.Code != "123456" || value.Expiry.Unix()%37 != 0 {
		t.Fatal("native session retained seed or used incorrect timing")
	}
}
