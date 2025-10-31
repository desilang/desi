package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM6P2_Inout_Requires_Lvalue(t *testing.T) {
	// pub def g(inout x:int) -> none; def main(): g(1)
	g := &ast.FuncDecl{
		Pub:     true,
		Name:    ast.Ident{Name: "g"},
		Params:  []ast.Param{{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamInout}},
		RetType: &ast.TypeName{Name: "none"},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.CallExpr{Callee: &ast.Ident{Name: "g"}, Args: []ast.Expr{&ast.IntLit{}}}},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{g, main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "mutable lvalue")
}

func TestM6P2_Inout_Aliasing(t *testing.T) {
	// pub def g(inout a:int, ref b:int) -> none; def main(): let x=0; g(x, x)
	g := &ast.FuncDecl{
		Pub:  true,
		Name: ast.Ident{Name: "g"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "a"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamInout},
			{Name: ast.Ident{Name: "b"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamRef},
		},
		RetType: &ast.TypeName{Name: "none"},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.LetStmt{Name: ast.Ident{Name: "x"}, Value: &ast.IntLit{}},
			&ast.ExprStmt{Expr: &ast.CallExpr{Callee: &ast.Ident{Name: "g"}, Args: []ast.Expr{&ast.Ident{Name: "x"}, &ast.Ident{Name: "x"}}}},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{g, main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "alias")
}

func TestM6P2_Use_After_Move(t *testing.T) {
	// pub def g(y:int) -> none; def main(): let t=1; g(t); t
	g := &ast.FuncDecl{
		Pub:     true,
		Name:    ast.Ident{Name: "g"},
		Params:  []ast.Param{{Name: ast.Ident{Name: "y"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamMove}},
		RetType: &ast.TypeName{Name: "none"},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.LetStmt{Name: ast.Ident{Name: "t"}, Value: &ast.IntLit{}},
			&ast.ExprStmt{Expr: &ast.CallExpr{Callee: &ast.Ident{Name: "g"}, Args: []ast.Expr{&ast.Ident{Name: "t"}}}},
			&ast.ExprStmt{Expr: &ast.Ident{Name: "t"}}, // use after move
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{g, main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "moved earlier")
}
