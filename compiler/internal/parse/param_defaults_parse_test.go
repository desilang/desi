package parse

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func renderParams(n ast.Node) string {
	var b strings.Builder
	ast.Print(&b, n)
	return strings.TrimSpace(b.String())
}

func TestFuncParamsWithDefaultsParse(t *testing.T) {
	src := strings.Join([]string{
		"def f(a: int = 1, b: int = 2):",
		"\treturn a",
		"",
	}, "\n")

	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}

	got := renderParams(mod)
	want := strings.TrimSpace(`
Module("<mem>")
	Func f(a: int = Int(1), b: int = Int(2))
		Block
			Return Ident(a)
`)
	if got != want {
		t.Fatalf("AST mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
