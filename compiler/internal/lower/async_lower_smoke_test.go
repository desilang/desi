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

	w, body := lower.LowerAsyncFunc(fn, nil, nil, nil)

	var buf bytes.Buffer

	// Wrapper: creates future, spawns body, returns future
	hir.Print(&buf, w)
	wout := buf.String()
	if !strings.Contains(wout, "func foo") {
		t.Fatalf("wrapper missing func header:\n%s", wout)
	}
	if !strings.Contains(wout, " = future.new") {
		t.Fatalf("wrapper missing future.new:\n%s", wout)
	}
	if !strings.Contains(wout, "future.spawn") {
		t.Fatalf("wrapper missing spawn call:\n%s", wout)
	}
	if !strings.Contains(wout, "ret %") {
		t.Fatalf("wrapper should return a future handle:\n%s", wout)
	}

	// Body: has __future__ param + original param, contains the function body
	buf.Reset()
	hir.Print(&buf, body)
	bout := buf.String()

	if !strings.Contains(bout, "func foo$body") {
		t.Fatalf("body missing func header:\n%s", bout)
	}
	if !strings.Contains(bout, "%__future__") {
		t.Fatalf("body missing __future__ param:\n%s", bout)
	}
	if !strings.Contains(bout, "%n") {
		t.Fatalf("body missing original param n:\n%s", bout)
	}
}
