package parse

import (
	"testing"
)

// For M14 stage-0 f-strings we only guarantee that `f"..."`
// is accepted by the lexer/parser and produces no diagnostics.
// There is no interpolation yet; the string behaves like a normal str.
func TestParse_FString_NoDiagnostics(t *testing.T) {
	src := `f"hello {world}"`

	mod, diags := ParseFile("f.desi", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", diags)
	}
	if mod == nil {
		t.Fatalf("expected non-nil module")
	}
}
