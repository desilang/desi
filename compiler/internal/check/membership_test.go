package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

func TestMembership_StrInStr_OK(t *testing.T) {
	// main: _ = "a" in "abc"
	be := &ast.BinaryExpr{
		Op:  "in",
		Lhs: &ast.StrLit{},
		Rhs: &ast.StrLit{},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: be}}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, info := Check(mod)
	mustNoDiags(t, diags)

	if got := info.Types[be]; got == nil || !types.Equal(got, types.Bool) {
		t.Fatalf("'in' type = %v, want bool", got)
	}
}

func TestMembership_Unsupported_DCO0002(t *testing.T) {
	// main: _ = 1 in "abc"
	be := &ast.BinaryExpr{
		Op:  "in",
		Lhs: &ast.IntLit{},
		Rhs: &ast.StrLit{},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: be}}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)

	found := false
	for _, d := range diags {
		if d.CodeID == "DCO0002" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected DCO0002 for unsupported membership, got %v diags", len(diags))
	}
}
