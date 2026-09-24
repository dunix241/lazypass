package app

import (
	"testing"

	"lazypass/internal/config"
)

func ptr[T any](value T) *T { return &value }

func TestResolveAppliesOnlyExplicitOverrides(t *testing.T) {
	saved := config.Defaults()
	saved.Length, saved.Symbols = 48, true
	resolved, err := Resolve(saved, Overrides{Length: ptr(128), Upper: ptr(false)})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Length != 128 || resolved.Upper || !resolved.Lower || !resolved.Numbers || !resolved.Symbols {
		t.Fatalf("unexpected resolved options: %+v", resolved)
	}
}

func TestResolveRejectsInvalidCombinedOptions(t *testing.T) {
	no := false
	_, err := Resolve(config.Defaults(), Overrides{Upper: &no, Lower: &no, Numbers: &no, Symbols: &no})
	if err == nil {
		t.Fatal("expected empty character set error")
	}
}

func TestGenerateReturnsRequestedMetadata(t *testing.T) {
	opts, err := Resolve(config.Defaults(), Overrides{Length: ptr(64)})
	if err != nil {
		t.Fatal(err)
	}
	results, err := Generate(opts, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results", len(results))
	}
	for _, result := range results {
		if len(result.Password) != 64 || result.Length != 64 || result.EntropyBits <= 0 || result.Strength == "" {
			t.Fatalf("unexpected result: %+v", result)
		}
	}
}
