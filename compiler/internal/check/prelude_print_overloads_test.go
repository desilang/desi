package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestPrelude_Print_Overloads_OK(t *testing.T) {
	// def main():
	//   print(1)
	//   print(1.0)
	//   print(true)
	//   print("hi")
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.CallExpr{
				Callee: &ast.Ident{Name: "print"},
				Args:   []ast.Expr{&ast.IntLit{}},
			}},
			&ast.ExprStmt{Expr: &ast.CallExpr{
				Callee: &ast.Ident{Name: "print"},
				Args:   []ast.Expr{&ast.FloatLit{}},
			}},
			&ast.ExprStmt{Expr: &ast.CallExpr{
				Callee: &ast.Ident{Name: "print"},
				Args:   []ast.Expr{&ast.BoolLit{Value: true}},
			}},
			&ast.ExprStmt{Expr: &ast.CallExpr{
				Callee: &ast.Ident{Name: "print"},
				Args:   []ast.Expr{&ast.StrLit{}},
			}},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}
