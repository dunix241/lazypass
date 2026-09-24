// Package generator is pure password generation with no UI deps.
// Uses crypto/rand with rejection sampling (no modulo bias) and guarantees
// at least one char from each enabled class when possible.
package generator

import (
	"crypto/rand"
	"errors"
	"math"
	"math/big"
)

const (
	MinLength = 4
	MaxLength = 256

	DefaultLength  = 20
	DefaultSymbols = "!@#$%^&*()-_=+[]{}<>?"

	upperLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerLetters = "abcdefghijklmnopqrstuvwxyz"
	digits       = "0123456789"
	ambiguous    = "Il1O0"
)

// Options describes what to generate. SymbolSet is used only when Symbols=true.
type Options struct {
	Length           int
	Upper            bool
	Lower            bool
	Numbers          bool
	Symbols          bool
	SymbolSet        string
	ExcludeAmbiguous bool
}

// Defaults matches REQUIREMENTS: 20 chars, upper+lower+numbers, no symbols.
func Defaults() Options {
	return Options{
		Length:           DefaultLength,
		Upper:            true,
		Lower:            true,
		Numbers:          true,
		Symbols:          false,
		SymbolSet:        DefaultSymbols,
		ExcludeAmbiguous: false,
	}
}

// Validate ensures length and at least one class.
func (o Options) Validate() error {
	if o.Length < MinLength || o.Length > MaxLength {
		return errors.New("length must be between 4 and 256")
	}
	if !o.Upper && !o.Lower && !o.Numbers && !o.Symbols {
		return errors.New("at least one character class must be enabled")
	}
	if o.Symbols && o.SymbolSet == "" {
		return errors.New("symbol set must not be empty when symbols are enabled")
	}
	return nil
}

// Pool returns the effective character pool after ambiguous filtering.
func (o Options) Pool() string {
	var pool string
	if o.Upper {
		pool += upperLetters
	}
	if o.Lower {
		pool += lowerLetters
	}
	if o.Numbers {
		pool += digits
	}
	if o.Symbols {
		pool += o.SymbolSet
	}
	return stripAmbiguous(pool, o.ExcludeAmbiguous)
}

// EntropyBits = length * log2(poolSize). Returns 0 when pool is empty.
func (o Options) EntropyBits() float64 {
	pool := o.Pool()
	if len(pool) == 0 {
		return 0
	}
	return float64(o.Length) * math.Log2(float64(len(pool)))
}

// StrengthLabel mirrors REQUIREMENTS thresholds.
func StrengthLabel(bits float64) string {
	switch {
	case bits < 40:
		return "Weak"
	case bits < 60:
		return "Fair"
	case bits < 80:
		return "Good"
	default:
		return "Strong"
	}
}

// Generate creates one password. Guarantees one char per enabled class
// when Length >= number of enabled classes.
func (o Options) Generate() (string, error) {
	if err := o.Validate(); err != nil {
		return "", err
	}
	pool := o.Pool()
	if len(pool) == 0 {
		return "", errors.New("empty character pool after filtering")
	}

	classes := o.enabledPools()
	out := make([]byte, 0, o.Length)

	// 1. One guaranteed char per class (if room).
	if o.Length >= len(classes) {
		for _, class := range classes {
			c, err := randByte(class)
			if err != nil {
				return "", err
			}
			out = append(out, c)
		}
	}
	// 2. Fill the rest from the full pool.
	for len(out) < o.Length {
		c, err := randByte(pool)
		if err != nil {
			return "", err
		}
		out = append(out, c)
	}
	// 3. Shuffle so guaranteed positions are not predictable.
	if err := shuffle(out); err != nil {
		return "", err
	}
	return string(out), nil
}

func (o Options) enabledPools() []string {
	var pools []string
	if o.Upper {
		pools = append(pools, stripAmbiguous(upperLetters, o.ExcludeAmbiguous))
	}
	if o.Lower {
		pools = append(pools, stripAmbiguous(lowerLetters, o.ExcludeAmbiguous))
	}
	if o.Numbers {
		pools = append(pools, stripAmbiguous(digits, o.ExcludeAmbiguous))
	}
	if o.Symbols {
		pools = append(pools, stripAmbiguous(o.SymbolSet, o.ExcludeAmbiguous))
	}
	// Drop any class that became empty after ambiguous filtering.
	kept := pools[:0]
	for _, p := range pools {
		if len(p) > 0 {
			kept = append(kept, p)
		}
	}
	return kept
}

func stripAmbiguous(s string, strip bool) string {
	if !strip {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if !containsByte(ambiguous, s[i]) {
			out = append(out, s[i])
		}
	}
	return string(out)
}

func containsByte(s string, b byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return true
		}
	}
	return false
}

// randByte picks uniformly via crypto/rand + rejection sampling (big.Int).
func randByte(pool string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(pool))))
	if err != nil {
		return 0, err
	}
	return pool[n.Int64()], nil
}

func shuffle(b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		b[i], b[int(j.Int64())] = b[int(j.Int64())], b[i]
	}
	return nil
}
