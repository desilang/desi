package lower_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/lower"
)

func TestM8H_AsyncLower_FrameSaveRestore(t *testing.T) {
	// async def foo() -> int:
	//   a = await A()
	//   b = await B(a)
	//
	// Thread-spawn model: body receives __future__ param,
	// executes sequentially (no frame save/restore needed).
	fd := &ast.FuncDecl{
		Name:  ast.Ident{Name: "foo"},
		Async: true,
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.AssignStmt{ // a := await A()
				LHS: []ast.Expr{&ast.Ident{Name: "a"}},
				RHS: []ast.Expr{&ast.UnaryExpr{Op: "await", X: &ast.CallExpr{Callee: &ast.Ident{Name: "A"}}}},
			},
			&ast.AssignStmt{ // b := await B(a)
				LHS: []ast.Expr{&ast.Ident{Name: "b"}},
				RHS: []ast.Expr{&ast.UnaryExpr{Op: "await", X: &ast.CallExpr{Callee: &ast.Ident{Name: "B"}, Args: []ast.Expr{&ast.Ident{Name: "a"}}}}},
			},
		}},
	}

	w, body := lower.LowerAsyncFunc(fd, nil, nil, nil)

	// Wrapper: creates future and spawns body (0 user params)
	var wbuf bytes.Buffer
	hir.Print(&wbuf, w)
	wout := wbuf.String()
	if !strings.Contains(wout, "func foo") {
		t.Fatalf("wrapper missing func header:\n%s", wout)
	}
	if !strings.Contains(wout, "future.spawn") {
		t.Fatalf("wrapper missing spawn call:\n%s", wout)
	}

	// Body: has __future__ param, executes A() and B(a) sequentially
	var bout bytes.Buffer
	hir.Print(&bout, body)
	out := bout.String()

	if !strings.Contains(out, "func foo$body(%__future__)") {
		t.Fatalf("body missing __future__ param in header:\n%s", out)
	}

	// Body should contain the await calls for A and B
	if !strings.Contains(out, "call A") {
		t.Fatalf("body missing call to A:\n%s", out)
	}
	if !strings.Contains(out, "call B") {
		t.Fatalf("body missing call to B:\n%s", out)
	}
}
