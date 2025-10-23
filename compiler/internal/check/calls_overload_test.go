package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// f(int)->int; f(float)->float; g(){ f(1); f(1.0) }
func TestM4_Calls_Overload_ExactMatch_OK(t *testing.T) {
	fInt := &ast.FuncDecl{
		Name:    ast.Ident{Name: "f"},
		Params:  []ast.Param{{Name: ast.Ident{Name: "x"}, Type: &ast.Ident{Name: "int"}}},
		RetType: &ast.Ident{Name: "int"},
		Body:    &ast.Block{},
	}
	fFloat := &ast.FuncDecl{
		Name:    ast.Ident{Name: "f"},
		Params:  []ast.Param{{Name: ast.Ident{Name: "x"}, Type: &ast.Ident{Name: "float"}}},
		RetType: &ast.Ident{Name: "float"},
		Body:    &ast.Block{},
	}
	g := &ast.FuncDecl{
		Name:    ast.Ident{Name: "g"},
		Params:  nil,
		RetType: nil,
		Body: &ast.Block{
			Stmts: []ast.Stmt{
				&ast.ExprStmt{Expr: &ast.CallExpr{Callee: &ast.Ident{Name: "f"}, Args: []ast.Expr{&ast.IntLit{}}}},
				&ast.ExprStmt{Expr: &ast.CallExpr{Callee: &ast.Ident{Name: "f"}, Args: []ast.Expr{&ast.FloatLit{}}}},
			},
		},
	}
	mod := &ast.Module{Filename: "<mem>", Decls: []ast.Decl{fInt, fFloat, g}}
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

// Only f(int)->int; call f(true) => no exact match
func TestM4_Calls_Overload_NoMatch(t *testing.T) {
	fInt := &ast.FuncDecl{
		Name:    ast.Ident{Name: "f"},
		Params:  []ast.Param{{Name: ast.Ident{Name: "x"}, Type: &ast.Ident{Name: "int"}}},
		RetType: &ast.Ident{Name: "int"},
		Body:    &ast.Block{},
	}
	h := &ast.FuncDecl{
		Name: "h",
		Body: &ast.Block{
			Stmts: []ast.Stmt{
				&ast.ExprStmt{Expr: &ast.CallExpr{Callee: &ast.Ident{Name: "f"}, Args: []ast.Expr{&ast.BoolLit{}}}},
			},
		},
	}
	mod := &ast.Module{Filename: "<mem>", Decls: []ast.Decl{fInt, h}}
	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "no matching overload")
}
