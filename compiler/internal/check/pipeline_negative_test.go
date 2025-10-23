package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM4_Pipeline_RHSNotCall(t *testing.T) {
	// 1 |> 2   (rhs is not a call)
	expr := &ast.BinaryExpr{
		Op:  "|>",
		Lhs: &ast.IntLit{},
		Rhs: &ast.IntLit{},
	}
	main := &ast.FuncDecl{Name: ast.Ident{Name: "main"}, Body: &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: expr}}}}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "pipeline expects a call")
}

func TestM4_Pipeline_CalleeNotIdent(t *testing.T) {
	// 1 |> ((x:int)=>x)(2)  -- callee is lambda, not identifier
	l := &ast.LambdaExpr{
		Params: []ast.LambdaParam{{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}}},
		Body:   &ast.IntLit{},
	}
	expr := &ast.BinaryExpr{
		Op:  "|>",
		Lhs: &ast.IntLit{},
		Rhs: &ast.CallExpr{
			Callee: l, // not an identifier
			Args:   []ast.Expr{&ast.IntLit{}},
		},
	}
	main := &ast.FuncDecl{Name: ast.Ident{Name: "main"}, Body: &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: expr}}}}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "pipeline target must be an identifier")
}
