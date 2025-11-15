package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// f(int) -> int, f(float) -> float; g(int, int) -> int
func buildOverloadModule() *ast.Module {
	fInt := &ast.FuncDecl{
		Name:    ast.Ident{Name: "f"},
		Params:  []ast.Param{{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}}},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	fFloat := &ast.FuncDecl{
		Name:    ast.Ident{Name: "f"},
		Params:  []ast.Param{{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "float"}}},
		RetType: &ast.TypeName{Name: "float"},
		Body:    &ast.Block{},
	}
	g := &ast.FuncDecl{
		Name: ast.Ident{Name: "g"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "a"}, Type: &ast.TypeName{Name: "int"}},
			{Name: ast.Ident{Name: "b"}, Type: &ast.TypeName{Name: "int"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}

	call1 := &ast.ExprStmt{Expr: &ast.CallExpr{
		Callee:   &ast.Ident{Name: "f"},
		ArgNodes: []ast.CallArg{{Expr: &ast.IntLit{}}},
	}}
	call2 := &ast.ExprStmt{Expr: &ast.CallExpr{
		Callee:   &ast.Ident{Name: "f"},
		ArgNodes: []ast.CallArg{{Expr: &ast.FloatLit{}}},
	}}
	call3 := &ast.ExprStmt{Expr: &ast.CallExpr{
		Callee: &ast.Ident{Name: "g"},
		ArgNodes: []ast.CallArg{
			{Expr: &ast.IntLit{}},
			{Expr: &ast.IntLit{}},
		},
	}}

	main := &ast.FuncDecl{
		Name:    ast.Ident{Name: "main"},
		Params:  nil,
		RetType: nil,
		Body:    &ast.Block{Stmts: []ast.Stmt{call1, call2, call3}},
	}
	return &ast.Module{File: "<mem>", Decls: []ast.Decl{fInt, fFloat, g, main}}
}

func TestM4_Calls_Overload_ExactMatch(t *testing.T) {
	mod := buildOverloadModule()
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

func TestM4_Calls_Overload_NoMatch(t *testing.T) {
	// Only f(int) and f(float) exist; h(int) is missing.
	h := &ast.ExprStmt{Expr: &ast.CallExpr{
		Callee: &ast.Ident{Name: "h"},
		ArgNodes: []ast.CallArg{
			{Expr: &ast.IntLit{}},
		},
	}}
	top := &ast.FuncDecl{
		Name:    ast.Ident{Name: "top"},
		Body:    &ast.Block{Stmts: []ast.Stmt{h}},
		RetType: nil,
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{top}}
	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "undefined")
}

func TestM4_Calls_Overload_ArityMismatch(t *testing.T) {
	// g expects two ints; here we pass one
	call := &ast.ExprStmt{Expr: &ast.CallExpr{
		Callee: &ast.Ident{Name: "g"},
		ArgNodes: []ast.CallArg{
			{Expr: &ast.IntLit{}},
		},
	}}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{
			Stmts: []ast.Stmt{call},
		},
	}
	// Include a valid g so arity mismatch can be diagnosed against it.
	g := &ast.FuncDecl{
		Name: ast.Ident{Name: "g"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "a"}, Type: &ast.TypeName{Name: "int"}},
			{Name: ast.Ident{Name: "b"}, Type: &ast.TypeName{Name: "int"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{g, main}}
	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "arity")
}
