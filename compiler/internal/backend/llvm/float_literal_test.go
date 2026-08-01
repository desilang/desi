package llvm

import (
	"strconv"
	"strings"
	"testing"
)

// LLVM's IR parser only reads a decimal floating-point constant when the
// mantissa carries a '.'. A literal written without one — `1e20`, `1E5`,
// `2e-3`, `3e8` — used to be passed through as written and the module was
// rejected with "integer constant must have integer type".
func TestLLVMDoubleAlwaysCarriesADecimalPoint(t *testing.T) {
	cases := []string{
		"1e20", "1E5", "2e-3", "3e8", "1e-9", "5",
		"1.5e10", "1.0e20", "6.022e23", "2.5", "0.1", "-2500000.5",
	}
	for _, in := range cases {
		got := llvmDouble(in)

		mantissa := got
		if i := strings.IndexAny(got, "eE"); i >= 0 {
			mantissa = got[:i]
		}
		if !strings.Contains(mantissa, ".") {
			t.Errorf("llvmDouble(%q) = %q: mantissa has no '.', LLVM will reject it", in, got)
			continue
		}

		// The value has to survive the rewrite exactly.
		want, err := strconv.ParseFloat(strings.ReplaceAll(in, "_", ""), 64)
		if err != nil {
			t.Fatalf("test case %q is not a float", in)
		}
		back, err := strconv.ParseFloat(got, 64)
		if err != nil {
			t.Errorf("llvmDouble(%q) = %q, which does not parse back: %v", in, got, err)
			continue
		}
		if back != want {
			t.Errorf("llvmDouble(%q) = %q: value changed, %v became %v", in, got, want, back)
		}
	}
}

// A literal that already spells out a decimal point is left exactly as written,
// so the IR of every program that compiled before this fix is unchanged.
func TestLLVMDoublePassesThroughDottedText(t *testing.T) {
	for _, in := range []string{"1.5e10", "2.5", "0.1", "-2500000.5", "1.0e20"} {
		if got := llvmDouble(in); got != in {
			t.Errorf("llvmDouble(%q) = %q, want it untouched", in, got)
		}
	}
}
