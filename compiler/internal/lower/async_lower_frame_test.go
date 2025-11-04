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
	// async def foo(n:int) -> int:
	//   a = await A(n)
	//   b = await B(a)
	fd := &ast.FuncDecl{
		Name:  &ast.Ident{Name: "foo"},
		Async: true,
		Body: &ast.Block{
			Stmts: []ast.Stmt{
				&ast.AssignStmt{ // a := await A(n)
					LHS: []ast.Expr{&ast.Ident{Name: "a"}},
					RHS: []ast.Expr{&ast.UnaryExpr{Op: "await", X: &ast.CallExpr{Callee: &ast.Ident{Name: "A"}}}},
				},
				&ast.AssignStmt{ // b := await B(a)
					LHS: []ast.Expr{&ast.Ident{Name: "b"}},
					RHS: []ast.Expr{&ast.UnaryExpr{Op: "await", X: &ast.CallExpr{Callee: &ast.Ident{Name: "B"}}}},
				},
			},
		},
	}

	w, p := lower.LowerAsyncFunc(fd, nil, nil)

	// Pretty-print poll HIR
	var pout bytes.Buffer
	hir.Print(&pout, p)

	out := pout.String()

	// Poll signature carries a frame param.
	if !strings.Contains(out, "func foo$poll(%frame)") {
		t.Fatalf("poll missing frame param in header:\n%s", out)
	}

	// We save before suspending (Tier-0: at least fut).
	if !strings.Contains(out, "frame.set frame.fut") {
		t.Fatalf("expected frame.set before suspend; got:\n%s", out)
	}

	// On resume, we restore (Tier-0: fut).
	if !strings.Contains(out, "= frame.get frame.fut") {
		t.Fatalf("expected frame.get on resume; got:\n%s", out)
	}

	// Ensure multiple states exist (two awaits + final).
	if !strings.Contains(out, "block state2") {
		t.Fatalf("expected state2 block in poll; got:\n%s", out)
	}

	_ = w
}
