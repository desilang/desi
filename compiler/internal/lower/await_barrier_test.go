package lower_test

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lower"
)

func asyncFD(name string, params []ast.Param, body ...ast.Stmt) *ast.FuncDecl {
	return &ast.FuncDecl{
		Async:  true,
		Name:   ast.Ident{Name: name},
		Params: params,
		Body:   &ast.Block{Stmts: body},
	}
}

func TestAwaitBarrier_Negative_InoutUsedAfterAwait(t *testing.T) {
	// async def f(x: inout T):
	//   y = await g(x)
	//   return x
	fd := asyncFD("f",
		[]ast.Param{{Name: ast.Ident{Name: "x"}, Mode: ast.ParamInout}},
		&ast.AssignStmt{
			LHS: []ast.Expr{&ast.Ident{Name: "y"}},
			RHS: []ast.Expr{&ast.UnaryExpr{Op: "await", X: &ast.CallExpr{Callee: &ast.Ident{Name: "g"}, Args: []ast.Expr{&ast.Ident{Name: "x"}}}}},
		},
		&ast.ReturnStmt{Value: &ast.Ident{Name: "x"}},
	)

	diags := lower.CheckAwaitBorrowBarrier(fd)
	if len(diags) == 0 {
		t.Fatalf("expected barrier diagnostic, got none")
	}
	if got := diags[0].CodeID; got != "DBR0001" {
		t.Fatalf("expected CodeID DBR0001, got %q", got)
	}
}

func TestAwaitBarrier_Positive_RefAcrossAwaitOK(t *testing.T) {
	// async def f(x: ref T):
	//   _ = await g()
	//   return x
	fd := asyncFD("f",
		[]ast.Param{{Name: ast.Ident{Name: "x"}, Mode: ast.ParamRef}},
		&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.CallExpr{Callee: &ast.Ident{Name: "g"}}}},
		&ast.ReturnStmt{Value: &ast.Ident{Name: "x"}},
	)
	diags := lower.CheckAwaitBorrowBarrier(fd)
	if len(diags) != 0 {
		t.Fatalf("did not expect barrier for ref across await; got %d diagnostics", len(diags))
	}
}

func TestAwaitBarrier_Positive_InoutNotUsedAfterAwaitOK(t *testing.T) {
	// async def f(x: inout T):
	//   _ = x
	//   _ = await g()
	fd := asyncFD("f",
		[]ast.Param{{Name: ast.Ident{Name: "x"}, Mode: ast.ParamInout}},
		&ast.ExprStmt{Expr: &ast.Ident{Name: "x"}},                                                        // use before await
		&ast.ExprStmt{Expr: &ast.UnaryExpr{Op: "await", X: &ast.CallExpr{Callee: &ast.Ident{Name: "g"}}}}, // await; no later use
	)
	diags := lower.CheckAwaitBorrowBarrier(fd)
	if len(diags) != 0 {
		t.Fatalf("did not expect barrier: inout not used after await; got %d diagnostics", len(diags))
	}
}
