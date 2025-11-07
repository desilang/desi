package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestLen_UnsupportedType_DCO0001(t *testing.T) {
	// main:
	//   _ = len(1)
	call := &ast.CallExpr{
		Callee: &ast.Ident{Name: "len"},
		Args:   []ast.Expr{&ast.IntLit{}},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: call}}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)

	found := false
	for _, d := range diags {
		if d.CodeID == "DCO0001" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected DCO0001 for len(int), got %v diags", len(diags))
	}
}
