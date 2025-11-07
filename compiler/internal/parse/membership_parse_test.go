package parse

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestParse_Membership_In_BinaryExpr_OK(t *testing.T) {
	src := `
def f():
	"a" in "abc"
`
	mod, pdiags := ParseFile("memb.desi", []byte(src))
	if len(pdiags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", pdiags)
	}
	// module → f → body[0] is an ExprStmt with BinaryExpr(op=="in")
	fd, ok := mod.Decls[0].(*ast.FuncDecl)
	if !ok || fd.Body == nil || len(fd.Body.Stmts) != 1 {
		t.Fatalf("unexpected AST shape for function body")
	}
	es, ok := fd.Body.Stmts[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("want ExprStmt")
	}
	be, ok := es.Expr.(*ast.BinaryExpr)
	if !ok || be.Op != "in" {
		t.Fatalf("want BinaryExpr(op==\"in\"), got %#v", es.Expr)
	}
}

func TestParse_Membership_In_OK(t *testing.T) {
	src := "def f():\n\t\"a\" in \"abc\"\n"
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}
	fd := mod.Decls[0].(*ast.FuncDecl)
	es := fd.Body.Stmts[0].(*ast.ExprStmt)
	if be, ok := es.Expr.(*ast.BinaryExpr); !ok || be.Op != "in" {
		t.Fatalf("want BinaryExpr(op==\"in\"), got %#v", es.Expr)
	}
}
