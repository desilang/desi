package resolve

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/parse"
)

func TestReexports_FromInitModule(t *testing.T) {
	ldr := NewMemLoader(map[string]string{
		"math/__mod.desi": `
from math.add import add
`,
		"math/add.desi": `
pub def add(a: int, b: int = 1) -> int: a + b
`,
		// NOTE: no synthetic __top__ here; parser will hoist top-level imports into __top__.
		"main.desi": `
from math import add
`,
	})

	// Parse the consumer
	mainMod, diags := parse.ParseFile("main.desi", []byte(ldr.Files["main.desi"]))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}

	// Resolve; this should load 'math', see the re-export, and surface it.
	diags2, info := Resolve(mainMod, ldr)
	if len(diags2) != 0 {
		t.Fatalf("unexpected resolve diags: %+v", diags2)
	}
	ex := info.ModuleExports["math"]
	if ex == nil {
		t.Fatalf("missing exports for math")
	}
	cands := ex.Funcs["add"]
	if len(cands) != 1 {
		t.Fatalf("expected 1 candidate for re-exported add, got %d", len(cands))
	}
	if got := cands[0].String(); got != "func(int, int) -> int" {
		t.Fatalf("re-exported add signature mismatch: %s", got)
	}

	// Default mask should also be preserved through the re-export.
	defs := ex.FuncDefaults["add"]
	if len(defs) != 1 {
		t.Fatalf("expected 1 defaults entry for re-exported add, got %d", len(defs))
	}
	if len(defs[0]) != 2 {
		t.Fatalf("expected 2 param-default flags, got %d", len(defs[0]))
	}
	if defs[0][0] {
		t.Fatalf("param a should not have a default")
	}
	if !defs[0][1] {
		t.Fatalf("param b should have a default")
	}

	_ = ast.Module{} // silence unused import if build tags change
}
