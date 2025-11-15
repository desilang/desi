package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/resolve"
	"github.com/desilang/desi/compiler/internal/types"
)

func TestPopulateImportedFuncSigs_Basic(t *testing.T) {
	ldr := resolve.NewMemLoader(map[string]string{
		"math/__mod.desi": `
pub def add(a: int, b: int = 1) -> int:
  return a + b
`,
	})

	mainSrc := `
from math import add as sum
`
	mod, diags := parse.ParseFile("main.desi", []byte(mainSrc))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}
	// Run resolver to build ModuleExports
	rdiags, rinfo := resolve.Resolve(mod, ldr)
	if len(rdiags) != 0 {
		t.Fatalf("unexpected resolve diags: %+v", rdiags)
	}

	info := NewInfo()
	PopulateImportedFuncSigs(mod, info, rinfo)

	set, ok := info.Funcs["sum"]
	if !ok || set == nil {
		t.Fatalf("missing overload set for 'sum'")
	}
	if len(set.Cands) != 1 {
		t.Fatalf("expected 1 cand for 'sum', got %d", len(set.Cands))
	}
	cand := set.Cands[0]

	// Signature matches
	want := types.FuncOf([]types.T{types.Int, types.Int}, types.Int)
	got := cand.Type
	if !types.Equal(want, got) {
		t.Fatalf("cand mismatch: want %s, got %s", want, got)
	}

	// Default mask propagated from resolver: a no default, b has default.
	if cand.Defaults == nil || len(cand.Defaults) != 2 {
		t.Fatalf("expected Defaults mask of length 2, got %#v", cand.Defaults)
	}
	if cand.Defaults[0] {
		t.Fatalf("param a should not have a default")
	}
	if !cand.Defaults[1] {
		t.Fatalf("param b should have a default")
	}
}

// Cross-module defaults must affect arity checking: add(a, b=1)
func TestM14_FromImport_Defaults_Call_OK(t *testing.T) {
	ldr := resolve.NewMemLoader(map[string]string{
		"math/__mod.desi": `
pub def add(a: int, b: int = 1) -> int:
  a + b
`,
		"main.desi": `
from math import add

def main() -> int:
  add(1)
  add(1, 3)
  0
`,
	})

	mod, diags := parse.ParseFile("main.desi", []byte(ldr.Files["main.desi"]))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	res := CheckWithLoader(mod, ldr)
	if len(res.Diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", res.Diags)
	}
}

// Too few args even with defaults should still be an arity error.
func TestM14_FromImport_Defaults_Call_TooFewArgs(t *testing.T) {
	ldr := resolve.NewMemLoader(map[string]string{
		"math/__mod.desi": `
pub def add(a: int, b: int = 1) -> int:
  a + b
`,
		"main.desi": `
from math import add

def main() -> int:
  add()
  0
`,
	})

	mod, diags := parse.ParseFile("main.desi", []byte(ldr.Files["main.desi"]))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	res := CheckWithLoader(mod, ldr)
	if len(res.Diags) == 0 {
		t.Fatalf("expected diagnostics, got none")
	}
	mustHaveSomeDiagContaining(t, res.Diags, "arity mismatch")
}
