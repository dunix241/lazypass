package service

import (
	"context"
	"time"

	"lazypass/internal/otp"
	"lazypass/internal/vault"
)

// Authenticator retains local key material only when generation is not delegated.
type Authenticator struct {
	key    *otp.Key
	period time.Duration
	Code   string
	Expiry time.Time
}

func (s Service) LoadAuthenticator(ctx context.Context, path vault.Path) (Authenticator, error) {
	entry, err := s.Show(ctx, path)
	if err != nil {
		return Authenticator{}, err
	}
	if entry.Type != vault.LoginEntry || entry.AuthenticatorUnavailable {
		return Authenticator{}, vault.ErrUnsupported
	}
	if entry.Authenticator == "" {
		return Authenticator{}, ErrEmptyCopy
	}
	key, err := otp.Parse(entry.Authenticator)
	if err != nil {
		return Authenticator{}, err
	}
	value := Authenticator{period: key.Period()}
	if _, native := s.Provider.(vault.CodeGenerator); !native {
		value.key = &key
	}
	err = s.RefreshAuthenticator(ctx, path, &value)
	return value, err
}

func (s Service) RefreshAuthenticator(ctx context.Context, path vault.Path, value *Authenticator) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if value.key != nil {
		code, expiry, err := value.key.Code(time.Now())
		if err == nil {
			value.Code, value.Expiry = code, expiry
		}
		return err
	}
	generator, ok := s.Provider.(vault.CodeGenerator)
	if !ok || value.period <= 0 {
		return vault.ErrUnsupported
	}
	// Discard a result that crossed rollover; its period cannot be identified.
	for attempt := 0; attempt < 2; attempt++ {
		start := time.Now()
		code, err := generator.AuthenticatorCode(ctx, path)
		if err != nil {
			return err
		}
		period := int64(value.period / time.Second)
		expiry := time.Unix(start.Unix()-start.Unix()%period+period, 0)
		if time.Now().Before(expiry) {
			value.Code, value.Expiry = code, expiry
			return nil
		}
	}
	return vault.ErrUnavailable
}

func (a Authenticator) Current(now time.Time) (string, int) {
	if a.Code == "" || !now.Before(a.Expiry) || now.Before(a.Expiry.Add(-a.period)) {
		return "", 0
	}
	return a.Code, int((a.Expiry.Sub(now) + time.Second - 1) / time.Second)
}
