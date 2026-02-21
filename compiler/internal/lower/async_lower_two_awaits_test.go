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

	w, body := lower.LowerAsyncFunc(fn, nil, nil, nil)

	var buf bytes.Buffer

	// Wrapper: creates future, spawns body with 1 param
	hir.Print(&buf, w)
	wout := buf.String()
	for _, want := range []string{
		"func foo",
		" = future.new",
		"future.spawn",
		"ret %",
	} {
		if !strings.Contains(wout, want) {
			t.Fatalf("wrapper missing %q in:\n%s", want, wout)
		}
	}

	// Body: has __future__ + n params, contains actual function body with awaits
	buf.Reset()
	hir.Print(&buf, body)
	bout := buf.String()

	wantLines := []string{
		"func foo$body",
		"%__future__",
		"%n",
	}
	for _, w := range wantLines {
		if !strings.Contains(bout, w) {
			t.Fatalf("body missing %q in:\n%s", w, bout)
		}
	}
}
