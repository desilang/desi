package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM4_NotCallable_Value(t *testing.T) {
	// (1)() -> expression is not callable
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{
			Stmts: []ast.Stmt{
				&ast.ExprStmt{Expr: &ast.CallExpr{
					Callee: &ast.IntLit{},
					Args:   nil,
				}},
			},
		},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}
	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "not callable")
}
