package check

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM6_Borrow_RefOnly_NoDiag(t *testing.T) {
	f := &ast.FuncDecl{
		Async: true,
		Name:  ast.Ident{Name: "g"},
		Params: []ast.Param{
			// ref-only param should NOT trigger borrow error across await
			{Name: ast.Ident{Name: "rx"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamRef},
			// awaitable param to keep type checker happy (use plain 'future')
			{Name: ast.Ident{Name: "p"}, Type: &ast.TypeName{Name: "future"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.Ident{Name: "p"}}},
			&ast.ExprStmt{Expr: &ast.Ident{Name: "rx"}},
		}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

func TestM6_Borrow_TwoAwaits_GivesTwoErrors(t *testing.T) {
	f := &ast.FuncDecl{
		Async: true,
		Name:  ast.Ident{Name: "h"},
		Params: []ast.Param{
			// inout param triggers the rule
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamInout},
			// two awaitable params (plain 'future')
			{Name: ast.Ident{Name: "p"}, Type: &ast.TypeName{Name: "future"}},
			{Name: ast.Ident{Name: "q"}, Type: &ast.TypeName{Name: "future"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.Ident{Name: "p"}}},
			&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.Ident{Name: "q"}}},
		}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)

	// Count DBR0001-ish messages by substring to avoid depending on code formatting.
	count := 0
	for _, d := range diags {
		if strings.Contains(d.Message, "cannot hold") && strings.Contains(d.Message, "await") {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 DBR0001 borrow diags (one per await), got %d (diags: %v)", count, diags)
	}
}
