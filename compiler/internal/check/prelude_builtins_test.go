package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestPrelude_Builtins_NoImports_OK(t *testing.T) {
	// main:
	//   print(str(42))
	//   _ = len("hi")
	//   _ = bool(0)
	callStr := &ast.CallExpr{
		Callee: &ast.Ident{Name: "str"},
		Args:   []ast.Expr{&ast.IntLit{}},
	}
	callPrint := &ast.ExprStmt{
		Expr: &ast.CallExpr{
			Callee: &ast.Ident{Name: "print"},
			Args:   []ast.Expr{callStr},
		},
	}
	callLen := &ast.ExprStmt{
		Expr: &ast.CallExpr{
			Callee: &ast.Ident{Name: "len"},
			Args:   []ast.Expr{&ast.StrLit{}}, // no Text field on StrLit
		},
	}
	callBool := &ast.ExprStmt{
		Expr: &ast.CallExpr{
			Callee: &ast.Ident{Name: "bool"},
			Args:   []ast.Expr{&ast.IntLit{}},
		},
	}

	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{callPrint, callLen, callBool}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

func TestPrelude_Shadowing_TopLevel_Function_Err(t *testing.T) {
	fd := &ast.FuncDecl{
		Name: ast.Ident{Name: "print"},
		Body: &ast.Block{},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{fd}}
	diags, _ := Check(mod)

	// Expect DPL0001
	found := false
	for _, d := range diags {
		if d.CodeID == "DPL0001" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected DPL0001 for shadowing builtin, got %v diags", len(diags))
	}
}
