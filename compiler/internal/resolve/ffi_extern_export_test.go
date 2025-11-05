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

	// sin: extern present, ABI should be "C", no link hint
	if got := len(exp.Funcs["sin"]); got != 1 {
		t.Fatalf("sin overloads mismatch: got %d", got)
	}
	{
		meta := exp.FuncExtern["sin"][0]
		if !meta.Extern {
			t.Fatalf("sin should be extern")
		}
		if meta.ABI != "C" {
			t.Fatalf("sin ABI expected C, got %q", meta.ABI)
		}
		if meta.Link != "" {
			t.Fatalf("sin link should be empty, got %q", meta.Link)
		}
	}

	// cos: extern present, ABI "C", link hint present (content not inspected in M9B)
	if got := len(exp.Funcs["cos"]); got != 1 {
		t.Fatalf("cos overloads mismatch: got %d", got)
	}
	{
		meta := exp.FuncExtern["cos"][0]
		if !meta.Extern {
			t.Fatalf("cos should be extern")
		}
		if meta.ABI != "C" {
			t.Fatalf("cos ABI expected C, got %q", meta.ABI)
		}
		if meta.Link == "" {
			t.Fatalf("cos link hint should be present (non-empty)")
		}
	}

	// add (non-extern)
	if got := len(exp.Funcs["add"]); got != 1 {
		t.Fatalf("add overloads mismatch: got %d", got)
	}
	if got := exp.FuncExtern["add"][0]; got.Extern {
		t.Fatalf("add should not be extern: %#v", got)
	}
}
