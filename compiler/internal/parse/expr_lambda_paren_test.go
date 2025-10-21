package parse

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func renderAST2(n ast.Node) string {
	var b strings.Builder
	ast.Print(&b, n)
	return strings.TrimSpace(b.String())
}

func TestLambdaParen_Forms(t *testing.T) {
	src := `
let a = (x:int, y:int) => x + y
let b = (x, y) => foo(x, y)
let c = (x:int, y:int,) => x
`
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if len(mod.Decls) != 1 {
		t.Fatalf("expected synthetic __top__ func hoisting lets, got %d decls", len(mod.Decls))
	}
	got := renderAST2(mod)

	wantSubs := []string{
		"Lambda(x: int, y: int) => (Ident(x) + Ident(y))",
		"Lambda(x, y) => Call Ident(foo)(Ident(x), Ident(y))",
		"Lambda(x: int, y: int) => Ident(x)",
	}
	for _, sub := range wantSubs {
		if !strings.Contains(got, sub) {
			t.Fatalf("AST render missing substring:\nwant contains: %s\n--- got ---\n%s", sub, got)
		}
	}
}
