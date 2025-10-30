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
pub def add(a: int, b: int) -> int:
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
	want := types.FuncOf([]types.T{types.Int, types.Int}, types.Int)
	got := set.Cands[0].Type
	if !types.Equal(want, got) {
		t.Fatalf("cand mismatch: want %s, got %s", want, got)
	}
}
