package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// Basic surface tests: ensure range(...) exists and is callable with 1/2/3 ints.
func TestRange_Surface_OK(t *testing.T) {
	f1 := &ast.FuncDecl{
		Name: ast.Ident{Name: "f1"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.CallExpr{
				Callee: &ast.Ident{Name: "range"},
				Args:   []ast.Expr{&ast.IntLit{}},
			}},
		}},
	}
	f2 := &ast.FuncDecl{
		Name: ast.Ident{Name: "f2"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.CallExpr{
				Callee: &ast.Ident{Name: "range"},
				Args:   []ast.Expr{&ast.IntLit{}, &ast.IntLit{}},
			}},
		}},
	}
	f3 := &ast.FuncDecl{
		Name: ast.Ident{Name: "f3"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.CallExpr{
				Callee: &ast.Ident{Name: "range"},
				Args:   []ast.Expr{&ast.IntLit{}, &ast.IntLit{}, &ast.IntLit{}},
			}},
		}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f1, f2, f3}}
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}
