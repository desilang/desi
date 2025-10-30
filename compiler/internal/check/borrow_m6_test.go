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
			// inout param triggers the borrow rule
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamInout},
			// awaitable param keeps type checker happy
			{Name: ast.Ident{Name: "p"}, Type: &ast.TypeName{Name: "future[int]"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.Ident{Name: "p"}}},
			&ast.ExprStmt{Expr: &ast.Ident{Name: "x"}},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)

	// Look for message text, not code literal.
	mustHaveSomeDiagContaining(t, diags, "cannot hold")
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
			{Name: ast.Ident{Name: "p"}, Type: &ast.TypeName{Name: "future[int]"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.Ident{Name: "p"}}},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "cannot hold")
	mustHaveSomeDiagContaining(t, diags, "x")
	mustHaveSomeDiagContaining(t, diags, "y")
}
