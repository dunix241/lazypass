package service

import (
	"context"
	"time"

	"lazypass/internal/otp"
	"lazypass/internal/vault"
)

// Authenticator retains key material only for the unlocked display session.
type Authenticator struct {
	key    *otp.Key
	period time.Duration
	Code   string
	Expiry time.Time
}

func (s Service) LoadAuthenticator(ctx context.Context, path vault.Path) (Authenticator, error) {
	if source, ok := s.Provider.(interface {
		AuthenticatorKey(context.Context, vault.Path) (otp.Key, error)
	}); ok {
		key, err := source.AuthenticatorKey(ctx, path)
		if err != nil {
			return Authenticator{}, err
		}
		value := Authenticator{key: &key, period: key.Period()}
		err = s.RefreshAuthenticator(ctx, path, &value)
		return value, err
	}
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
	if a.Code != "" && now.Before(a.Expiry) && !now.Before(a.Expiry.Add(-a.period)) {
		return a.Code, remainingSeconds(a.Expiry, now)
	}
	if a.key != nil {
		code, expiry, err := a.key.Code(now)
		if err != nil {
			return "", 0
		}
		return code, remainingSeconds(expiry, now)
	}
	return "", 0
}

func (a *Authenticator) UpdateLocal(now time.Time) {
	if a.key == nil || (a.Code != "" && now.Before(a.Expiry) && !now.Before(a.Expiry.Add(-a.period))) {
		return
	}
	code, expiry, err := a.key.Code(now)
	if err == nil {
		a.Code, a.Expiry = code, expiry
	}
}

func remainingSeconds(expiry, now time.Time) int {
	return int((expiry.Sub(now) + time.Second - 1) / time.Second)
}
