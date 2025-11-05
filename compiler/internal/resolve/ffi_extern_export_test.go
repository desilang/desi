package resolve

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
)

func TestExternExports_Metadata_AlignsWithOverloads(t *testing.T) {
	src := `
@extern("C")
pub def sin(x: float) -> float

@extern("C", "m")
pub def cos(x: float) -> float

# control (non-extern)
pub def add(a: int, b: int) -> int: a + b
`
	mod, diags := parse.ParseFile("ffi.desi", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", diags)
	}

	exp := CollectExports(mod)
	if exp == nil {
		t.Fatalf("nil exports")
	}

	// sin
	if got := len(exp.Funcs["sin"]); got != 1 {
		t.Fatalf("sin overloads mismatch: got %d", got)
	}
	if got := exp.FuncExtern["sin"][0]; !got.Extern || got.ABI != "C" || got.Link != "" {
		t.Fatalf("sin extern meta mismatch: %#v", got)
	}

	// cos
	if got := len(exp.Funcs["cos"]); got != 1 {
		t.Fatalf("cos overloads mismatch: got %d", got)
	}
	if got := exp.FuncExtern["cos"][0]; !got.Extern || got.ABI != "C" || got.Link != "m" {
		t.Fatalf("cos extern meta mismatch: %#v", got)
	}

	// add (non-extern)
	if got := len(exp.Funcs["add"]); got != 1 {
		t.Fatalf("add overloads mismatch: got %d", got)
	}
	if got := exp.FuncExtern["add"][0]; got.Extern {
		t.Fatalf("add should not be extern: %#v", got)
	}
}
