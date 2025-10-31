package check

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// This test ensures that having only 'ref' params does NOT trigger our borrow rule.
// We purposely do NOT assert "no diagnostics" because the type checker may
// complain about the await operand; we only assert that our borrow error does not appear.
func TestM6_Borrow_RefOnly_NoBorrowDiag(t *testing.T) {
	f := &ast.FuncDecl{
		Async: true,
		Name:  ast.Ident{Name: "g"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "rx"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamRef},
			// Keep this simple; its exact type isn't important for this test.
			{Name: ast.Ident{Name: "p"}, Type: &ast.TypeName{Name: "future[int]"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.Ident{Name: "p"}}},
			&ast.ExprStmt{Expr: &ast.Ident{Name: "rx"}},
		}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)

	for _, d := range diags {
		if strings.Contains(d.Message, "cannot hold") && strings.Contains(d.Message, "await") {
			t.Fatalf("unexpected borrow error for ref-only params: %v", d)
		}
	}
}

func TestM6_Borrow_TwoAwaits_GivesTwoErrors(t *testing.T) {
	f := &ast.FuncDecl{
		Async: true,
		Name:  ast.Ident{Name: "h"},
		Params: []ast.Param{
			// inout param triggers the rule
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamInout},
			// two awaitable-ish params; other type diags are fine, we count only borrow errors
			{Name: ast.Ident{Name: "p"}, Type: &ast.TypeName{Name: "future[int]"}},
			{Name: ast.Ident{Name: "q"}, Type: &ast.TypeName{Name: "future[int]"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.Ident{Name: "p"}}},
			&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.Ident{Name: "q"}}},
		}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)

	// Count only our borrow error messages; ignore unrelated type errors.
	count := 0
	for _, d := range diags {
		if strings.Contains(d.Message, "cannot hold") && strings.Contains(d.Message, "await") {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 borrow errors (one per await), got %d (diags: %v)", count, diags)
	}
}
