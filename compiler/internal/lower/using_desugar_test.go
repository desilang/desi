package lower_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/lower"
)

func TestLower_UsingDesugar_DestroyRunsOnEarlyReturn(t *testing.T) {
	// using arena:
	//   let x = arena.alloc(1)
	//   if true: return
	using := &ast.UsingStmt{
		Bind: &ast.Ident{Name: "arena"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.LetStmt{
				Name:  ast.Ident{Name: "x"},
				Value: &ast.CallExpr{Callee: &ast.FieldExpr{X: &ast.Ident{Name: "arena"}, Name: ast.Ident{Name: "alloc"}}, Args: []ast.Expr{&ast.IntLit{Text: "1"}}},
			},
			&ast.IfStmt{
				Cond: &ast.BoolLit{Value: true},
				Then: &ast.Block{Stmts: []ast.Stmt{&ast.ReturnStmt{}}},
			},
		}},
	}
	blk := &ast.Block{Stmts: []ast.Stmt{using}}
	fn := lower.LowerBlock("test", blk)

	var buf bytes.Buffer
	hir.Print(&buf, fn)
	out := buf.String()

	// We expect destroy_arena to be called on the return path.
	// Note: It might also be called on the fall-through path (dead code in this case), so count can be 2.
	if cnt := strings.Count(out, "destroy_arena arena"); cnt < 1 {
		t.Fatalf("expected at least one destroy_arena arena; got %d\n%s", cnt, out)
	}
	// it must appear before ret
	iDestroy := strings.Index(out, "destroy_arena arena")
	iRet := strings.Index(out, "ret")
	if !(iDestroy != -1 && iRet != -1 && iDestroy < iRet) {
		t.Fatalf("expected 'destroy_arena arena' before 'ret'\n%s", out)
	}
}
