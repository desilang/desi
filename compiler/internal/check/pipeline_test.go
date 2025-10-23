package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM4_Pipeline_Ok(t *testing.T) {
	add := &ast.FuncDecl{
		Name: ast.Ident{Name: "add"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "a"}, Type: &ast.TypeName{Name: "int"}},
			{Name: ast.Ident{Name: "b"}, Type: &ast.TypeName{Name: "int"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	// 4 |> add(5)  ==> add(4,5)
	expr := &ast.BinaryExpr{
		Op:  "|>",
		Lhs: &ast.IntLit{},
		Rhs: &ast.CallExpr{
			Callee: &ast.Ident{Name: "add"},
			Args:   []ast.Expr{&ast.IntLit{}},
		},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{
			Stmts: []ast.Stmt{&ast.ExprStmt{Expr: expr}},
		},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{add, main}}
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

func TestM4_Pipeline_NoMatch(t *testing.T) {
	add := &ast.FuncDecl{
		Name: ast.Ident{Name: "add"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "a"}, Type: &ast.TypeName{Name: "int"}},
			{Name: ast.Ident{Name: "b"}, Type: &ast.TypeName{Name: "int"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	// 4 |> add()  ==> tries add(4) -> no matching overload for pipeline call
	expr := &ast.BinaryExpr{
		Op:  "|>",
		Lhs: &ast.IntLit{},
		Rhs: &ast.CallExpr{
			Callee: &ast.Ident{Name: "add"},
			Args:   []ast.Expr{},
		},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{
			Stmts: []ast.Stmt{&ast.ExprStmt{Expr: expr}},
		},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{add, main}}
	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "pipeline")
}
