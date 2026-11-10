package service

import (
	"context"
	"time"

	"lazypass/internal/otp"
	"lazypass/internal/vault"
)

func (s Service) CopyAuthenticator(ctx context.Context, path vault.Path, guards ...context.Context) error {
	if s.Copy == nil {
		return ErrClipboard
	}
	p, err := s.provider()
	if err != nil {
		return err
	}
	code, err := s.authenticatorCode(ctx, p, path)
	if err != nil {
		return err
	}
	return s.copyValue(ctx, code, guards...)
}

func (s Service) authenticatorCode(ctx context.Context, p vault.Provider, path vault.Path) (string, error) {
	if generator, ok := p.(vault.CodeGenerator); ok {
		return generator.AuthenticatorCode(ctx, path)
	}

	entry, err := s.Show(ctx, path)
	if err != nil {
		return "", err
	}
	if entry.Type != vault.LoginEntry || entry.AuthenticatorUnavailable {
		return "", vault.ErrUnsupported
	}
	if entry.Authenticator == "" {
		return "", ErrEmptyCopy
	}

	key, err := otp.Parse(entry.Authenticator)
	if err != nil {
		return "", err
	}
	code, expiry, err := key.Code(time.Now())
	if err == nil && !time.Now().Before(expiry) {
		code, _, err = key.Code(time.Now())
	}
	return code, err
}
