package parse

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func renderAST(n ast.Node) string {
	var b strings.Builder
	ast.Print(&b, n)
	return strings.TrimSpace(b.String())
}

func TestLambdaAndComprehensions_Parse(t *testing.T) {
	src := `
let g = x => x*x
let evens = [x for x in range(10) if x % 2 == 0]
let pairs = {k: v for k in items if v > 0}
let s = #{x*x for x in xs}
`
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if len(mod.Decls) != 1 {
		t.Fatalf("expected synthetic __top__ func hoisting these lets, got %d decls", len(mod.Decls))
	}
	_ = mod.Decls[0].(*ast.FuncDecl) // structure sanity check

	got := renderAST(mod)

	wantSubs := []string{
		"Lambda(x) => (Ident(x) * Ident(x))",
		"[Ident(x) for Ident(x) in Call Ident(range)(Int(10)) if ((Ident(x) % Int(2)) == Int(0))]",
		"{Ident(k): Ident(v) for Ident(k) in Ident(items) if (Ident(v) > Int(0))}",
		"#{(Ident(x) * Ident(x)) for Ident(x) in Ident(xs)}",
	}
	for _, sub := range wantSubs {
		if !strings.Contains(got, sub) {
			t.Fatalf("AST render missing substring:\nwant contains: %s\n--- got ---\n%s", sub, got)
		}
	}
}
