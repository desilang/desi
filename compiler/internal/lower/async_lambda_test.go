package lower_test

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lower"
)

func TestAsyncLambda_DesugarsToHiddenAsyncFunc(t *testing.T) {
	// (async lambda x: await inc(x))(41)  ==>  __lam$0(41)
	call := &ast.CallExpr{
		Callee: &ast.LambdaExpr{
			Async:  true,
			Params: []ast.Param{{Name: ast.Ident{Name: "x"}}},
			Body: &ast.UnaryExpr{
				Op: "await",
				X: &ast.CallExpr{
					Callee: &ast.Ident{Name: "inc"},
					Args:   []ast.Expr{&ast.Ident{Name: "x"}},
				},
			},
		},
		Args: []ast.Expr{&ast.IntLit{Text: "41"}},
	}

	fn := &ast.FuncDecl{
		Name:  ast.Ident{Name: "top"},
		Async: false,
		Body:  &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: call}}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{fn}}

	lower.DesugarAsyncLambdas(mod)

	// 1) The call's callee must now be an identifier __lam$N
	es, ok := fn.Body.Stmts[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("stmt[0] not ExprStmt")
	}
	c2, ok := es.Expr.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expr not CallExpr")
	}
	id, ok := c2.Callee.(*ast.Ident)
	if !ok {
		t.Fatalf("callee not Ident after desugar: %#v", c2.Callee)
	}
	if !strings.HasPrefix(id.Name, "__lam$") {
		t.Fatalf("callee ident name %q does not start with __lam$", id.Name)
	}

	// 2) A corresponding hidden async function must have been synthesized.
	found := false
	for _, d := range mod.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			if fd.Name.Name == id.Name && fd.Async {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("no synthesized async function %q found in module", id.Name)
	}
}
