package generator

import (
	"strings"
	"testing"
)

func TestDefaultsValid(t *testing.T) {
	if err := Defaults().Validate(); err != nil {
		t.Fatalf("defaults must be valid: %v", err)
	}
}

func TestValidateLengthBounds(t *testing.T) {
	for _, l := range []int{0, 1, 3, 257, 1000} {
		o := Defaults()
		o.Length = l
		if err := o.Validate(); err == nil {
			t.Fatalf("length %d should fail validation", l)
		}
	}
	for _, l := range []int{4, 20, 50, 51, 256} {
		o := Defaults()
		o.Length = l
		if err := o.Validate(); err != nil {
			t.Fatalf("length %d should pass: %v", l, err)
		}
	}
}

func TestValidateNeedsOneClass(t *testing.T) {
	o := Defaults()
	o.Upper, o.Lower, o.Numbers, o.Symbols = false, false, false, false
	if err := o.Validate(); err == nil {
		t.Fatal("all classes disabled should fail")
	}
}

func TestGenerateLengthAndCharset(t *testing.T) {
	o := Defaults()
	o.Length = 32
	pw, err := o.Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(pw) != 32 {
		t.Fatalf("got len %d, want 32", len(pw))
	}
	pool := o.Pool()
	for i := 0; i < len(pw); i++ {
		if !strings.ContainsRune(pool, rune(pw[i])) {
			t.Fatalf("char %q not in pool", pw[i])
		}
	}
}

func TestGenerateGuaranteesEachClass(t *testing.T) {
	o := Options{Length: 20, Upper: true, Lower: true, Numbers: true, Symbols: true, SymbolSet: DefaultSymbols}
	for i := 0; i < 50; i++ {
		pw, err := o.Generate()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if !strings.ContainsAny(pw, upperLetters) {
			t.Fatal("missing uppercase")
		}
		if !strings.ContainsAny(pw, lowerLetters) {
			t.Fatal("missing lowercase")
		}
		if !strings.ContainsAny(pw, digits) {
			t.Fatal("missing digit")
		}
	}
}

func TestExcludeAmbiguous(t *testing.T) {
	o := Defaults()
	o.ExcludeAmbiguous = true
	o.Length = 64
	pw, err := o.Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if strings.ContainsAny(pw, ambiguous) {
		t.Fatalf("ambiguous char found in %q", pw)
	}
}

func TestLongLengthNoCap(t *testing.T) {
	// Regression for LastPass 50-char limit: we must support >50.
	o := Defaults()
	o.Length = 200
	pw, err := o.Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(pw) != 200 {
		t.Fatalf("got len %d, want 200", len(pw))
	}
}

func TestEntropyAndStrength(t *testing.T) {
	o := Defaults()
	if bits := o.EntropyBits(); bits <= 0 {
		t.Fatalf("entropy must be positive, got %v", bits)
	}
	if got := StrengthLabel(20); got != "Weak" {
		t.Fatalf("got %q want Weak", got)
	}
	if got := StrengthLabel(100); got != "Strong" {
		t.Fatalf("got %q want Strong", got)
	}
}

func BenchmarkGenerate(b *testing.B) {
	o := Defaults()
	o.Length = 32
	for i := 0; i < b.N; i++ {
		if _, err := o.Generate(); err != nil {
			b.Fatal(err)
		}
	}
}
