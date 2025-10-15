package parse

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func renderDecor(n ast.Node) string {
	var b strings.Builder
	ast.Print(&b, n)
	return strings.TrimSpace(b.String())
}

func TestDecoratorsAndDocstringAttach(t *testing.T) {
	src := strings.Join([]string{
		"@bench",
		"@pkg.Deco(1, 2)",
		"def f(x: int) -> int:",
		"\t\"\"\"hello\"\"\"",
		"\treturn x",
		"",
	}, "\n")

	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}

	got := renderDecor(mod)
	// We expect:
	//   @bench
	//   @pkg.Deco(Int(1), Int(2))
	//   Func f(x: int) -> int
	//     DocString
	//     Block
	//       Return Ident(x)
	want := strings.TrimSpace(`
Module("<mem>")
	@bench
	@pkg.Deco(Int(1), Int(2))
	Func f(x: int) -> int
		DocString
		Block
			Return Ident(x)
`)
	if got != want {
		t.Fatalf("AST mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
