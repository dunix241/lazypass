// Package otp validates provisioning inputs and generates local TOTP codes.
package otp

import (
	"encoding/base32"
	"errors"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	lib "github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	MaxInput  = 8192
	maxPeriod = 86400
)

var ErrInvalid = errors.New("invalid authenticator: use a Base32 key or supported TOTP URI")

type Key struct {
	secret    string
	period    uint
	digits    lib.Digits
	algorithm lib.Algorithm
	uri       string
}

// Parse rejects ambiguous parameters and unsupported formats without echoing input.
func Parse(value string) (Key, error) {
	k := Key{period: 30, digits: lib.DigitsSix, algorithm: lib.AlgorithmSHA1}
	if len(value) > MaxInput || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return Key{}, ErrInvalid
	}
	value = strings.TrimSpace(value)
	secret := value
	if strings.Contains(value, ":") {
		q, err := parseURI(value)
		if err != nil {
			return Key{}, ErrInvalid
		}
		secret = q.Get("secret")
		switch strings.ToUpper(q.Get("algorithm")) {
		case "", "SHA1":
		case "SHA256":
			k.algorithm = lib.AlgorithmSHA256
		case "SHA512":
			k.algorithm = lib.AlgorithmSHA512
		default:
			return Key{}, ErrInvalid
		}
		switch q.Get("digits") {
		case "", "6":
		case "8":
			k.digits = lib.DigitsEight
		default:
			return Key{}, ErrInvalid
		}
		if p := q.Get("period"); p != "" {
			if strings.IndexFunc(p, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
				return Key{}, ErrInvalid
			}
			n, err := strconv.ParseUint(p, 10, 32)
			if err != nil || n == 0 || n > maxPeriod {
				return Key{}, ErrInvalid
			}
			k.period = uint(n)
		}
		k.uri = value
	}
	secret = strings.ToUpper(secret)
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	if strings.Contains(secret, "=") {
		encoding = base32.StdEncoding
	}
	decoded, err := encoding.DecodeString(secret)
	defer clear(decoded)
	if err != nil || len(decoded) == 0 {
		return Key{}, ErrInvalid
	}
	k.secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(decoded)
	return k, nil
}

func parseURI(value string) (url.Values, error) {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "otpauth" || u.Host != "totp" || u.User != nil || strings.Contains(value, "#") || u.Opaque != "" {
		return nil, ErrInvalid
	}
	label := strings.TrimPrefix(u.Path, "/")
	if strings.TrimSpace(label) == "" || strings.IndexFunc(label, unicode.IsControl) >= 0 {
		return nil, ErrInvalid
	}
	issuer, account, hasIssuer := strings.Cut(label, ":")
	if hasIssuer && (issuer == "" || strings.TrimSpace(account) == "" || strings.Contains(account, ":")) {
		return nil, ErrInvalid
	}

	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, ErrInvalid
	}
	for name, values := range query {
		if len(values) != 1 || values[0] == "" || strings.IndexFunc(values[0], unicode.IsControl) >= 0 {
			return nil, ErrInvalid
		}
		switch name {
		case "secret", "issuer", "algorithm", "digits", "period":
		default:
			return nil, ErrInvalid
		}
	}
	if hasIssuer && query.Get("issuer") != "" && issuer != query.Get("issuer") {
		return nil, ErrInvalid
	}
	return query, nil
}

// Normalize stores plain keys as interoperable URIs; existing URI labels survive.
func Normalize(value, label string) (string, error) {
	if value == "" {
		return "", nil
	}
	k, err := Parse(value)
	if err != nil {
		return "", err
	}
	if k.uri != "" {
		return k.uri, nil
	}
	if label == "" {
		label = "Login"
	}
	label = strings.Map(func(r rune) rune {
		if r == ':' || unicode.IsControl(r) {
			return '-'
		}
		return r
	}, label)
	u := url.URL{Scheme: "otpauth", Host: "totp", Path: "/" + label}
	u.RawQuery = "secret=" + k.secret
	return u.String(), nil
}

func (k Key) Code(now time.Time) (string, time.Time, error) {
	if now.Unix() < 0 || k.period == 0 || now.Unix() > math.MaxInt64-int64(k.period) {
		return "", time.Time{}, ErrInvalid
	}
	code, err := totp.GenerateCodeCustom(k.secret, now, totp.ValidateOpts{
		Period:    k.period,
		Digits:    k.digits,
		Algorithm: k.algorithm,
	})
	if err != nil {
		return "", time.Time{}, ErrInvalid
	}
	period := int64(k.period)
	expires := time.Unix(now.Unix()-now.Unix()%period+period, 0)
	return code, expires, nil
}
