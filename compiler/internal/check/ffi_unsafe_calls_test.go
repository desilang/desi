package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
)

func TestExternCall_RequiresUnsafe_Diag(t *testing.T) {
	src := `
@extern("C", "m")
pub def cadd(x: int, y: int) -> int:
  # extern decl; body intentionally empty

def f() -> int:
  cadd(1, 2)
  0
`
	mod, pdiags := parse.ParseFile("<mem>", []byte(src))
	if len(pdiags) != 0 {
		t.Fatalf("parse diags: %+v", pdiags)
	}
	res := Check(mod, nil /* loader will be wired by resolve inside Check */)
	for _, d := range res.Diags {
		if d.CodeID == "DFI0003" {
			return // found expected diag
		}
	}
	t.Fatalf("expected DFI0003 for extern call in safe context, got: %+v", res.Diags)
}

func TestExternCall_AllowedInsideUnsafe(t *testing.T) {
	src := `
@extern("C", "m")
pub def cadd(x: int, y: int) -> int:

def f() -> int:
  unsafe:
    cadd(1, 2)
  0
`
	mod, pdiags := parse.ParseFile("<mem>", []byte(src))
	if len(pdiags) != 0 {
		t.Fatalf("parse diags: %+v", pdiags)
	}
	res := Check(mod, nil)
	for _, d := range res.Diags {
		if d.CodeID == "DFI0003" {
			t.Fatalf("unexpected DFI0003 inside unsafe: %+v", res.Diags)
		}
	}
}
