package parse

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func renderNamedArgs(n ast.Node) string {
	var b strings.Builder
	ast.Print(&b, n)
	return strings.TrimSpace(b.String())
}

// Ensure the parser builds CallExpr.ArgNodes with names and the AST printer
// renders named arguments in the expected order/shape.
func TestNamedArgs_ParseAndPrint(t *testing.T) {
	src := strings.Join([]string{
		"def f(x: int, y: int, z: int):",
		"\treturn x + y + z",
		"",
		"def g():",
		"\tf(1, y=2, z=3)",
		"\tf(x=10, y=20, z=30)",
		"",
	}, "\n")

	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	if mod == nil {
		t.Fatalf("expected non-nil module")
	}

	got := renderNamedArgs(mod)

	wantSubs := []string{
		// mixed positional + named
		"Call Ident(f)(Int(1), y=Int(2), z=Int(3))",
		// all named
		"Call Ident(f)(x=Int(10), y=Int(20), z=Int(30))",
	}
	for _, sub := range wantSubs {
		if !strings.Contains(got, sub) {
			t.Fatalf("AST render missing substring:\nwant contains: %s\n--- got ---\n%s", sub, got)
		}
	}
}
