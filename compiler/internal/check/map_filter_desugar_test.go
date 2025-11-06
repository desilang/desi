package check

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func render(n ast.Node) string {
	var b strings.Builder
	ast.Print(&b, n)
	return strings.TrimSpace(b.String())
}

func TestDesugar_Map_ToListComp(t *testing.T) {
	// main: map(xs, f)
	call := &ast.CallExpr{
		Callee: &ast.Ident{Name: "map"},
		Args:   []ast.Expr{&ast.Ident{Name: "xs"}, &ast.Ident{Name: "f"}},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: call}}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustNoDiags(t, diags)

	// After desugaring, the body should contain a ListComp:
	// [ Call f(__x) for __x in xs ]
	got := render(main.Body.Stmts[0])
	want := "[ Call f(__x) for __x in xs]"
	if got != want {
		t.Fatalf("desugared AST mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestDesugar_Filter_ToListComp(t *testing.T) {
	// main: filter(xs, p)
	call := &ast.CallExpr{
		Callee: &ast.Ident{Name: "filter"},
		Args:   []ast.Expr{&ast.Ident{Name: "xs"}, &ast.Ident{Name: "p"}},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: call}}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustNoDiags(t, diags)

	// [ __x for __x in xs if Call p(__x) ]
	got := render(main.Body.Stmts[0])
	want := "[ __x for __x in xs if Call p(__x)]"
	if got != want {
		t.Fatalf("desugared AST mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
