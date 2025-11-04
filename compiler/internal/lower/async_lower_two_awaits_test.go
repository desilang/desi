package lower_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/lower"
)

func TestAsyncLower_TwoAwaits_StateMachineShape(t *testing.T) {
	// async def foo(n:int) -> int:
	//   x = await a(n)
	//   y = await b(x)
	//   return y
	fn := &ast.FuncDecl{
		Async:   true,
		Name:    ast.Ident{Name: "foo"},
		Params:  []ast.Param{{Name: ast.Ident{Name: "n"}, Type: &ast.TypeName{Name: "int"}}},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.AssignStmt{
				LHS: []ast.Expr{&ast.Ident{Name: "x"}},
				RHS: []ast.Expr{&ast.UnaryExpr{
					Op: "await",
					X: &ast.CallExpr{
						Callee: &ast.Ident{Name: "a"},
						Args:   []ast.Expr{&ast.Ident{Name: "n"}},
					},
				}},
			},
			&ast.AssignStmt{
				LHS: []ast.Expr{&ast.Ident{Name: "y"}},
				RHS: []ast.Expr{&ast.UnaryExpr{
					Op: "await",
					X: &ast.CallExpr{
						Callee: &ast.Ident{Name: "b"},
						Args:   []ast.Expr{&ast.Ident{Name: "x"}},
					},
				}},
			},
			&ast.ReturnStmt{Value: &ast.Ident{Name: "y"}},
		}},
	}

	w, p := lower.LowerAsyncFunc(fn, nil, nil)

	var buf bytes.Buffer
	// Wrapper sanity
	hir.Print(&buf, w)
	wout := buf.String()
	for _, want := range []string{
		"func foo",
		" = future.new",
		"ret %",
	} {
		if !strings.Contains(wout, want) {
			t.Fatalf("wrapper missing %q in:\n%s", want, wout)
		}
	}

	// Poll should contain a multi-state dispatch and three states (0,1 -> pending; 2 -> complete)
	buf.Reset()
	hir.Print(&buf, p)
	pout := buf.String()

	wantLines := []string{
		"func foo$poll",
		"  block entry",
		// at least the first comparison
		"call __eq_i32(frame.state, 0)",
		"if %t", // generic shape
		// state blocks
		"  block state0",
		"    frame.state = 1",
		"    ret false",
		"  block state1",
		"    frame.state = 2",
		"    ret false",
		"  block state2",
		"    future.complete frame.fut, 0",
		"    ret true",
	}
	for _, w := range wantLines {
		if !strings.Contains(pout, w) {
			t.Fatalf("poll missing line %q in:\n%s", w, pout)
		}
	}
}
