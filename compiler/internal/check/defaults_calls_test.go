package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// f(a: int, b: int = 2) -> int; calling f(1) should be OK
func TestM14_Defaults_Call_PositionalTrailing(t *testing.T) {
	f := &ast.FuncDecl{
		Name: ast.Ident{Name: "f"},
		Params: []ast.Param{
			{
				Name: ast.Ident{Name: "a"},
				Type: &ast.TypeName{Name: "int"},
			},
			{
				Name:    ast.Ident{Name: "b"},
				Type:    &ast.TypeName{Name: "int"},
				Default: &ast.IntLit{}, // defaulted param
			},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	call := &ast.ExprStmt{
		Expr: &ast.CallExpr{
			Callee: &ast.Ident{Name: "f"},
			Args:   []ast.Expr{&ast.IntLit{}}, // only 'a' provided; 'b' via default
		},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{
			Stmts: []ast.Stmt{call},
		},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f, main}}

	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

// g(a: int, b: int) -> int; calling g(1) should still be an arity error
func TestM14_Defaults_Call_PositionalMissingRequired(t *testing.T) {
	g := &ast.FuncDecl{
		Name: ast.Ident{Name: "g"},
		Params: []ast.Param{
			{
				Name: ast.Ident{Name: "a"},
				Type: &ast.TypeName{Name: "int"},
			},
			{
				Name: ast.Ident{Name: "b"},
				Type: &ast.TypeName{Name: "int"},
			},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	call := &ast.ExprStmt{
		Expr: &ast.CallExpr{
			Callee: &ast.Ident{Name: "g"},
			Args:   []ast.Expr{&ast.IntLit{}}, // missing 'b'
		},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{
			Stmts: []ast.Stmt{call},
		},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{g, main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "arity mismatch")
}

// Overloads:
//
//	f(a: int) -> int
//	f(a: int, b: int = 1) -> int
//
// Call f(1) should be ambiguous: both are viable after applying defaults.
func TestM14_Defaults_Call_AmbiguousWithDefaults(t *testing.T) {
	f1 := &ast.FuncDecl{
		Name: ast.Ident{Name: "f"},
		Params: []ast.Param{
			{
				Name: ast.Ident{Name: "a"},
				Type: &ast.TypeName{Name: "int"},
			},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	f2 := &ast.FuncDecl{
		Name: ast.Ident{Name: "f"},
		Params: []ast.Param{
			{
				Name: ast.Ident{Name: "a"},
				Type: &ast.TypeName{Name: "int"},
			},
			{
				Name:    ast.Ident{Name: "b"},
				Type:    &ast.TypeName{Name: "int"},
				Default: &ast.IntLit{},
			},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	call := &ast.ExprStmt{
		Expr: &ast.CallExpr{
			Callee: &ast.Ident{Name: "f"},
			Args:   []ast.Expr{&ast.IntLit{}},
		},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{
			Stmts: []ast.Stmt{call},
		},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f1, f2, main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "ambiguous overload")
}
