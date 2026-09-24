// Package app owns transport-neutral password generation policy.
package app

import (
	"fmt"

	"lazypass/internal/config"
	"lazypass/internal/generator"
)

// Overrides contains only explicitly supplied option changes.
type Overrides struct {
	Length           *int
	Upper            *bool
	Lower            *bool
	Numbers          *bool
	Symbols          *bool
	SymbolSet        *string
	ExcludeAmbiguous *bool
}

// Result is generation metadata shared by CLI and TUI transports.
type Result struct {
	Password    string  `json:"password"`
	Length      int     `json:"length"`
	EntropyBits float64 `json:"entropyBits"`
	Strength    string  `json:"strength"`
}

// Resolve applies explicit overrides to saved options and validates the result.
func Resolve(saved config.Config, overrides Overrides) (generator.Options, error) {
	opts := saved.ToOptions()
	if overrides.Length != nil {
		opts.Length = *overrides.Length
	}
	if overrides.Upper != nil {
		opts.Upper = *overrides.Upper
	}
	if overrides.Lower != nil {
		opts.Lower = *overrides.Lower
	}
	if overrides.Numbers != nil {
		opts.Numbers = *overrides.Numbers
	}
	if overrides.Symbols != nil {
		opts.Symbols = *overrides.Symbols
	}
	if overrides.SymbolSet != nil {
		opts.SymbolSet = *overrides.SymbolSet
	}
	if overrides.ExcludeAmbiguous != nil {
		opts.ExcludeAmbiguous = *overrides.ExcludeAmbiguous
	}
	if err := opts.Validate(); err != nil {
		return generator.Options{}, err
	}
	return opts, nil
}

// Generate returns count independent passwords from already-resolved options.
func Generate(opts generator.Options, count int) ([]Result, error) {
	if count < 1 {
		return nil, fmt.Errorf("count must be at least 1")
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	results := make([]Result, 0, count)
	for range count {
		password, err := opts.Generate()
		if err != nil {
			return nil, err
		}
		results = append(results, Result{
			Password: password, Length: opts.Length, EntropyBits: opts.EntropyBits(), Strength: generator.StrengthLabel(opts.EntropyBits()),
		})
	}
	return results, nil
}
