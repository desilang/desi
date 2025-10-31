package check

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

func TestM6P2_LastUse_AwaitAfterLastUse_OK(t *testing.T) {
	f := &ast.FuncDecl{
		Async: true,
		Name:  ast.Ident{Name: "f"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamInout},
			{Name: ast.Ident{Name: "p"}, Type: &ast.TypeName{Name: "future[int]"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.Ident{Name: "x"}}, // last use BEFORE await
			&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.Ident{Name: "p"}}},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)
	// should NOT contain our borrow error
	mustNotContain(t, diags, "cannot hold")
}

func TestM6P2_LastUse_AwaitBeforeLastUse_Err(t *testing.T) {
	f := &ast.FuncDecl{
		Async: true,
		Name:  ast.Ident{Name: "f"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamInout},
			{Name: ast.Ident{Name: "p"}, Type: &ast.TypeName{Name: "future[int]"}},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.Ident{Name: "p"}}}, // await BEFORE last use
			&ast.ExprStmt{Expr: &ast.Ident{Name: "x"}},                                 // last use
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "cannot hold")
}

// helper: assert all diag messages lack a substring
func mustNotContain(t *testing.T, diags []diag.Diagnostic, sub string) {
	t.Helper()
	for _, d := range diags {
		if strings.Contains(d.Message, sub) {
			t.Fatalf("unexpected diagnostic containing %q: %v", sub, d)
		}
	}
}
