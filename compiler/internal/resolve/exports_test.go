package resolve

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
)

func TestCollectExports_TypedOnly(t *testing.T) {
	src := `
pub def add(a: int, b: int) -> int:
  return a + b

def bad(a):
  return a

pub def area(r: float) -> float:
  return r

def priv(a: int) -> int:
  return a
`
	mod, diags := parse.ParseFile("m.desi", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", diags)
	}

	exp := CollectExports(mod)
	if exp == nil {
		t.Fatalf("nil exports")
	}
	if len(exp.Funcs) != 2 {
		t.Fatalf("expected 2 exported names, got %d (Funcs=%v)", len(exp.Funcs), exp.Funcs)
	}
	if c := exp.Funcs["add"]; len(c) != 1 {
		t.Fatalf("expected add to have 1 candidate, got %d", len(c))
	} else if got := c[0].String(); got != "func(int, int) -> int" {
		t.Fatalf("add candidate mismatch: got %q", got)
	}
	if c := exp.Funcs["area"]; len(c) != 1 {
		t.Fatalf("expected area to have 1 candidate, got %d", len(c))
	} else if got := c[0].String(); got != "func(float) -> float" {
		t.Fatalf("area candidate mismatch: got %q", got)
	}
	if _, ok := exp.Funcs["bad"]; ok {
		t.Fatalf("bad should not be exported (untyped param)")
	}
	if _, ok := exp.Funcs["priv"]; ok {
		t.Fatalf("priv should not be exported (not pub)")
	}
}
