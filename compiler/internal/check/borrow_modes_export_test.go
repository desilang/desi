package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/resolve"
)

// This file verifies that exported param modes (from resolver) are observed by the checker
// when the chosen candidate has no local Decl (i.e., chosen.Decl == nil and only Modes are available).

func TestM6_ModesExport_FromImport_RefRequiresLvalue(t *testing.T) {
	// utilx.__mod.desi exports: pub def view(ref x: int) -> int
	ldr := resolve.NewMemLoader(map[string]string{
		"utilx/__mod.desi": `
pub def view(ref x: int) -> int:
  return 0
`,
	})

	// main: from utilx import view; def main(): view(1)
	src := `
from utilx import view

def main():
  view(1)
`
	mod, pdiags := parse.ParseFile("main.desi", []byte(src))
	if len(pdiags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", pdiags)
	}

	res := CheckWithLoader(mod, ldr)
	// Cross-module: chosen.Decl == nil, Modes come from resolver export → DBR0005 fires.
	mustHaveSomeDiagContaining(t, res.Diags, "lvalue")
}

func TestM6_ModesExport_ImportQualified_InoutRef_Aliasing(t *testing.T) {
	// utilx.__mod.desi exports: pub def touch(inout a: int, ref b: int) -> int
	ldr := resolve.NewMemLoader(map[string]string{
		"utilx/__mod.desi": `
pub def touch(inout a: int, ref b: int) -> int:
  return 0
`,
	})

	// main: import utilx; def main(): let x = 0; utilx.touch(x, x)
	src := `
import utilx

def main():
  let x = 0
  utilx.touch(x, x)
`
	mod, pdiags := parse.ParseFile("main.desi", []byte(src))
	if len(pdiags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", pdiags)
	}

	res := CheckWithLoader(mod, ldr)
	// Cross-module: DBR0003 aliasing should be enforced using exported Modes.
	mustHaveSomeDiagContaining(t, res.Diags, "alias")
}
