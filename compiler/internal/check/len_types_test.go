package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

func TestLen_Str_ReturnsUsize(t *testing.T) {
	call := &ast.CallExpr{
		Callee: &ast.Ident{Name: "len"},
		Args:   []ast.Expr{&ast.StrLit{}}, // StrLit has no Text field
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: call}}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, info := Check(mod)
	mustNoDiags(t, diags)

	if got := info.Types[call]; got == nil || !types.Equal(got, types.USize) {
		t.Fatalf("len(str) type = %v, want usize", got)
	}
}
