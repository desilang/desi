package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// Verify the stubs are visible and accept an lvalue for the first (inout) arg.
func TestPrelude_CollectionsStubs_LvalueOK(t *testing.T) {
	// def foo(xs):
	//   list_push(xs, 1)
	foo := &ast.FuncDecl{
		Name: ast.Ident{Name: "foo"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "xs"}}, // untyped param; acts as an lvalue
		},
		Body: &ast.Block{
			Stmts: []ast.Stmt{
				&ast.ExprStmt{
					Expr: &ast.CallExpr{
						Callee: &ast.Ident{Name: "list_push"},
						Args:   []ast.Expr{&ast.Ident{Name: "xs"}, &ast.IntLit{}},
					},
				},
			},
		},
	}

	// def bar(s):
	//   set_add(s, "x")
	bar := &ast.FuncDecl{
		Name: ast.Ident{Name: "bar"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "s"}},
		},
		Body: &ast.Block{
			Stmts: []ast.Stmt{
				&ast.ExprStmt{
					Expr: &ast.CallExpr{
						Callee: &ast.Ident{Name: "set_add"},
						Args:   []ast.Expr{&ast.Ident{Name: "s"}, &ast.StrLit{}},
					},
				},
			},
		},
	}

	// def baz(m):
	//   dict_set(m, "k", 1)
	baz := &ast.FuncDecl{
		Name: ast.Ident{Name: "baz"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "m"}},
		},
		Body: &ast.Block{
			Stmts: []ast.Stmt{
				&ast.ExprStmt{
					Expr: &ast.CallExpr{
						Callee: &ast.Ident{Name: "dict_set"},
						Args:   []ast.Expr{&ast.Ident{Name: "m"}, &ast.StrLit{}, &ast.IntLit{}},
					},
				},
			},
		},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{foo, bar, baz}}
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}
