package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/resolve"
	"github.com/desilang/desi/compiler/internal/types"
)

// M14 integration tests: defaults + named args across modules (imports + re-exports).

func TestM14_Defaults_Import_Direct_OK(t *testing.T) {
	ldr := resolve.NewMemLoader(map[string]string{
		"math/__mod.desi": `
pub def lerp(start: float, end: float, t: float = 0.5) -> float:
  return start + (end - start) * t
`,
	})

	mainSrc := `
from math import lerp

def main() -> int:
  # positional only, uses default t
  lerp(0.0, 10.0)
  # named, all provided
  lerp(start=0.0, end=10.0, t=0.25)
  # mix positional + named, still uses default t
  lerp(0.0, end=10.0)
  return 0
`
	mod, diags := parse.ParseFile("main_direct.desi", []byte(mainSrc))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	res := CheckWithLoader(mod, ldr)
	if len(res.Diags) != 0 {
		t.Fatalf("unexpected check diags: %+v", res.Diags)
	}

	// Sanity: ensure we actually recorded a callable for lerp in Info.
	set := res.Info.Funcs["lerp"]
	if set == nil || len(set.Cands) != 1 {
		t.Fatalf("expected 1 overload for lerp, got %#v", set)
	}
	want := types.FuncOf([]types.T{types.Float, types.Float, types.Float}, types.Float)
	if got := set.Cands[0].Type; !types.Equal(got, want) {
		t.Fatalf("lerp signature mismatch: want %s, got %s", want, got)
	}
}

func TestM14_Defaults_Import_Reexport_OK(t *testing.T) {
	ldr := resolve.NewMemLoader(map[string]string{
		"math/__mod.desi": `
pub def lerp(start: float, end: float, t: float = 0.5) -> float:
  return start + (end - start) * t
`,
		"util/__mod.desi": `
from math import lerp
`,
	})

	mainSrc := `
from util import lerp as ulerp

def main() -> int:
  # positional only via re-export, uses default t
  ulerp(0.0, 10.0)
  # named via re-export, all provided
  ulerp(start=0.0, end=10.0, t=0.25)
  # mix positional + named via re-export, uses default t
  ulerp(0.0, end=10.0)
  return 0
`
	mod, diags := parse.ParseFile("main_reexport.desi", []byte(mainSrc))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	res := CheckWithLoader(mod, ldr)
	if len(res.Diags) != 0 {
		t.Fatalf("unexpected check diags: %+v", res.Diags)
	}

	// Ensure we have a properly-typed overload for ulerp coming from the re-export.
	set := res.Info.Funcs["ulerp"]
	if set == nil || len(set.Cands) != 1 {
		t.Fatalf("expected 1 overload for ulerp, got %#v", set)
	}
	want := types.FuncOf([]types.T{types.Float, types.Float, types.Float}, types.Float)
	if got := set.Cands[0].Type; !types.Equal(got, want) {
		t.Fatalf("ulerp signature mismatch: want %s, got %s", want, got)
	}
}

func TestM14_Defaults_Import_UnknownNamed(t *testing.T) {
	ldr := resolve.NewMemLoader(map[string]string{
		"math/__mod.desi": `
pub def lerp(start: float, end: float, t: float = 0.5) -> float:
  return start + (end - start) * t
`,
	})

	mainSrc := `
from math import lerp

def main() -> int:
  # bad names: should trigger DCA0001 on 'foo'
  lerp(foo=0.0, bar=10.0)
  return 0
`
	mod, diags := parse.ParseFile("main_bad_named.desi", []byte(mainSrc))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	res := CheckWithLoader(mod, ldr)
	if len(res.Diags) == 0 {
		t.Fatalf("expected diagnostics, got none")
	}
	mustHaveSomeDiagContaining(t, res.Diags, "unknown named argument")
}
