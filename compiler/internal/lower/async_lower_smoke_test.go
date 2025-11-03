package lower_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/lower"
)

func TestAsyncLower_Smoke_WrapperAndPoll(t *testing.T) {
	// async def foo(n: int) -> int:
	//   x = 1
	//   y = await bar(n)
	//   return x + y
	fn := &ast.FuncDecl{
		Async:   true,
		Name:    ast.Ident{Name: "foo"},
		Params:  []ast.Param{{Name: ast.Ident{Name: "n"}, Type: &ast.TypeName{Name: "int"}}},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.AssignStmt{
				LHS: []ast.Expr{&ast.Ident{Name: "x"}},
				RHS: []ast.Expr{&ast.IntLit{Text: "1"}},
			},
			&ast.AssignStmt{
				LHS: []ast.Expr{&ast.Ident{Name: "y"}},
				RHS: []ast.Expr{&ast.UnaryExpr{
					Op: "await",
					X: &ast.CallExpr{
						Callee: &ast.Ident{Name: "bar"},
						Args:   []ast.Expr{&ast.Ident{Name: "n"}},
					},
				}},
			},
			&ast.ReturnStmt{
				Value: &ast.BinaryExpr{Op: "+", Lhs: &ast.Ident{Name: "x"}, Rhs: &ast.Ident{Name: "y"}},
			},
		}},
	}

	w, p := lower.LowerAsyncFunc(fn, nil, nil)

	var buf bytes.Buffer

	// Wrapper expectations
	hir.Print(&buf, w)
	wout := buf.String()
	if !strings.Contains(wout, "func foo") {
		t.Fatalf("wrapper missing func header:\n%s", wout)
	}
	if !strings.Contains(wout, " = future.new") {
		t.Fatalf("wrapper missing future.new:\n%s", wout)
	}
	if !strings.Contains(wout, "ret %") {
		t.Fatalf("wrapper should return a future handle:\n%s", wout)
	}

	// Poll expectations
	buf.Reset()
	hir.Print(&buf, p)
	pout := buf.String()

	want := []string{
		"func foo$poll",
		"  block entry",
		"    %t1 = call __eq_i32(frame.state, 0)",
		"    if %t1 then state0 else state1",
		"  block state0",
		"    frame.state = 1",
		"    ret false",
		"  block state1",
		"    future.complete frame.fut, 0",
		"    ret true",
	}
	for _, line := range want {
		if !strings.Contains(pout, line) {
			t.Fatalf("poll missing line %q in:\n%s", line, pout)
		}
	}
}
