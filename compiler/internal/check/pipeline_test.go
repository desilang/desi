package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// add(a:int, b:int) -> int
func buildAddModule() *ast.Module {
	add := &ast.FuncDecl{
		Name: ast.Ident{Name: "add"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "a"}, Type: &ast.TypeName{Name: "int"}},
			{Name: ast.Ident{Name: "b"}, Type: &ast.TypeName{Name: "int"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	return &ast.Module{File: "<mem>", Decls: []ast.Decl{add}}
}

func TestM4_Pipeline_Ok(t *testing.T) {
	mod := buildAddModule()

	// main: 1 |> add(2)
	pipe := &ast.ExprStmt{Expr: &ast.BinaryExpr{
		Op:  "|>",
		Lhs: &ast.IntLit{},
		Rhs: &ast.CallExpr{
			Callee: &ast.Ident{Name: "add"},
			Args:   []ast.Expr{&ast.IntLit{}},
		},
	}}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{pipe}},
	}
	mod.Decls = append(mod.Decls, main)

	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

func TestM4_Pipeline_ArityMismatch(t *testing.T) {
	mod := buildAddModule()

	// main: 1 |> add()   // only one total arg (from pipe), add needs 2
	pipe := &ast.ExprStmt{Expr: &ast.BinaryExpr{
		Op:  "|>",
		Lhs: &ast.IntLit{},
		Rhs: &ast.CallExpr{
			Callee: &ast.Ident{Name: "add"},
			Args:   nil,
		},
	}}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{pipe}},
	}
	mod.Decls = append(mod.Decls, main)

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "arity mismatch")
}

func TestM4_Pipeline_TypeMismatch(t *testing.T) {
	mod := buildAddModule()

	// main: 1.0 |> add(2)  // float piped into add(int,int)
	pipe := &ast.ExprStmt{Expr: &ast.BinaryExpr{
		Op:  "|>",
		Lhs: &ast.FloatLit{},
		Rhs: &ast.CallExpr{
			Callee: &ast.Ident{Name: "add"},
			Args:   []ast.Expr{&ast.IntLit{}},
		},
	}}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{pipe}},
	}
	mod.Decls = append(mod.Decls, main)

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "no matching overload")
}
