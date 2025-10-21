package parse

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func renderAST3(n ast.Node) string {
	var b strings.Builder
	ast.Print(&b, n)
	return strings.TrimSpace(b.String())
}

func TestComprehension_Chains(t *testing.T) {
	src := `
let L = [x+y for x in xs if p(x) for y in ys if q(y)]
let S = #{f(x) for x in xs if ok(x)}
let D = {k: v for k in ks for v in vs if v>0}
`
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	got := renderAST3(mod)
	wantSubs := []string{
		"[(Ident(x) + Ident(y)) for Ident(x) in Ident(xs) if Call Ident(p)(Ident(x)) for Ident(y) in Ident(ys) if Call Ident(q)(Ident(y))]",
		"#{Call Ident(f)(Ident(x)) for Ident(x) in Ident(xs) if Call Ident(ok)(Ident(x))}",
		"{Ident(k): Ident(v) for Ident(k) in Ident(ks) for Ident(v) in Ident(vs) if (Ident(v) > Int(0))}",
	}
	for _, sub := range wantSubs {
		if !strings.Contains(got, sub) {
			t.Fatalf("AST render missing substring:\nwant contains: %s\n--- got ---\n%s", sub, got)
		}
	}
}
