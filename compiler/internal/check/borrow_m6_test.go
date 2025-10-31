package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM6_Borrow_NoAwait(t *testing.T) {
	f := &ast.FuncDecl{
		Async: true,
		Name:  ast.Ident{Name: "f"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamInout},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.Ident{Name: "x"}},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

func TestM6_Borrow_WithAwait(t *testing.T) {
	f := &ast.FuncDecl{
		Async: true,
		Name:  ast.Ident{Name: "f"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamInout},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.UnaryExpr{
				Op: "await",
				X:  &ast.CallExpr{Callee: &ast.Ident{Name: "sleep"}, Args: []ast.Expr{&ast.IntLit{}}},
			}},
			&ast.ExprStmt{Expr: &ast.Ident{Name: "x"}},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "DBR0001")
	mustHaveSomeDiagContaining(t, diags, "inout")
	mustHaveSomeDiagContaining(t, diags, "await")
}

func TestM6_Borrow_MultiInout(t *testing.T) {
	f := &ast.FuncDecl{
		Async: true,
		Name:  ast.Ident{Name: "f"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamInout},
			{Name: ast.Ident{Name: "y"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamInout},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.Ident{Name: "foo"}}},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "DBR0001")
	mustHaveSomeDiagContaining(t, diags, "x")
	mustHaveSomeDiagContaining(t, diags, "y")
}
