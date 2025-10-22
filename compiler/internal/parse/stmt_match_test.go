package parse

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestMatch_Parse_ValueArms(t *testing.T) {
	src := `
def tier(x: int) -> str:
	match x:
		x < 0: "neg"
		0: "zero"
		x < 10: "small"
		_: "big"
`
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %+v", diags)
	}
	var b strings.Builder
	ast.Print(&b, mod)
	got := b.String()
	checks := []string{
		"Match Ident(x)",
		`Case (Ident(x) < Int(0)): Str("neg")`,
		`Case Int(0): Str("zero")`,
		`Case (Ident(x) < Int(10)): Str("small")`,
		`Case Ident(_): Str("big")`,
	}
	for _, c := range checks {
		if !strings.Contains(got, c) {
			t.Fatalf("AST render missing substring:\nwant contains: %s\n--- got ---\n%s", c, got)
		}
	}
}
