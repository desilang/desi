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
pub def add(a: int, b: int) -> int: a + b
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

	_ = ast.Module{} // silence unused import if build tags change
}
