package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func Test_baseAndPath(t *testing.T) {
	x := &ast.Ident{Name: "x"}
	if b, p, ok := baseAndPath(x); !ok || b != "x" || p != "" {
		t.Fatalf("ident: got (%q,%q,%v)", b, p, ok)
	}

	f := &ast.FieldExpr{X: x, Name: ast.Ident{Name: "f"}}
	if b, p, ok := baseAndPath(f); !ok || b != "x" || p != ".f" {
		t.Fatalf("field: got (%q,%q,%v)", b, p, ok)
	}

	i := &ast.IndexExpr{X: x, Idx: &ast.IntLit{Text: "0"}}
	if b, p, ok := baseAndPath(i); !ok || b != "x" || p != "[]" {
		t.Fatalf("index: got (%q,%q,%v)", b, p, ok)
	}

	nest := &ast.IndexExpr{X: f, Idx: &ast.IntLit{Text: "1"}}
	if b, p, ok := baseAndPath(nest); !ok || b != "x" || p != ".f[]" {
		t.Fatalf("nested: got (%q,%q,%v)", b, p, ok)
	}
}
