package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

func TestM4_Lambda_TypedParams_OK(t *testing.T) {
	l := &ast.LambdaExpr{
		Params: []ast.LambdaParam{
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}},
		},
		RetType: &ast.TypeName{Name: "int"}, // Required explicit return type
		// Body avoids referencing param to keep M4 simple
		Body: &ast.IntLit{},
	}
	let := &ast.LetStmt{Name: ast.Ident{Name: "f"}, Type: &ast.TypeName{Name: "func"}, Value: l}
	main := &ast.FuncDecl{Name: ast.Ident{Name: "main"}, Body: &ast.Block{Stmts: []ast.Stmt{let}}}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, info := Check(mod)
	mustNoDiags(t, diags)

	got := info.Types[l]
	want := types.FuncOf([]types.T{types.Int}, types.Int, false)
	if !types.Equal(got, want) {
		t.Fatalf("lambda type mismatch: got %v, want %v", got, want)
	}
}

func TestM4_Lambda_UntypedParams_Error(t *testing.T) {
	l := &ast.LambdaExpr{
		Params: []ast.LambdaParam{
			{Name: ast.Ident{Name: "x"}, Type: nil}, // untyped param should trigger M4 error
		},
		RetType: &ast.TypeName{Name: "int"}, // Include return type
		Body:    &ast.IntLit{},
	}
	let := &ast.LetStmt{Name: ast.Ident{Name: "f"}, Type: &ast.TypeName{Name: "func"}, Value: l}
	main := &ast.FuncDecl{Name: ast.Ident{Name: "main"}, Body: &ast.Block{Stmts: []ast.Stmt{let}}}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "lambda parameters must be typed")
}

func TestM4_Lambda_Call_Direct_OK(t *testing.T) {
	l := &ast.LambdaExpr{
		Params: []ast.LambdaParam{
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}},
		},
		RetType: &ast.TypeName{Name: "int"}, // Required explicit return type
		Body:    &ast.IntLit{},
	}
	call := &ast.ExprStmt{Expr: &ast.CallExpr{
		Callee: l,
		Args:   []ast.Expr{&ast.IntLit{}}, // arity=1, arg=int
	}}
	main := &ast.FuncDecl{Name: ast.Ident{Name: "main"}, Body: &ast.Block{Stmts: []ast.Stmt{call}}}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}
