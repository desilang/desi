package lower_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/lower"
)

func TestLower_ArenaScope_SingleDestroy_NoPerAllocDrops(t *testing.T) {
	// using arena:
	//   let x = arena.alloc(1)
	//   let y = arena.alloc(2)
	//   return
	using := &ast.UsingStmt{
		Bind: &ast.Ident{Name: "arena"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.LetStmt{
				Name:  ast.Ident{Name: "x"},
				Value: &ast.CallExpr{Callee: &ast.FieldExpr{X: &ast.Ident{Name: "arena"}, Name: ast.Ident{Name: "alloc"}}, Args: []ast.Expr{&ast.IntLit{Text: "1"}}},
			},
			&ast.LetStmt{
				Name:  ast.Ident{Name: "y"},
				Value: &ast.CallExpr{Callee: &ast.FieldExpr{X: &ast.Ident{Name: "arena"}, Name: ast.Ident{Name: "alloc"}}, Args: []ast.Expr{&ast.IntLit{Text: "2"}}},
			},
			&ast.ReturnStmt{},
		}},
	}
	blk := &ast.Block{Stmts: []ast.Stmt{using}}
	fn := lower.LowerBlock("test", blk)

	var buf bytes.Buffer
	hir.Print(&buf, fn)
	out := buf.String()

	// Two allocs…
	if cnt := strings.Count(out, "arena.alloc("); cnt != 2 {
		t.Fatalf("expected 2 arena.alloc calls, got %d\n%s", cnt, out)
	}
	// …no drops of x or y…
	if strings.Contains(out, "drop x") || strings.Contains(out, "drop y") {
		t.Fatalf("unexpected per-allocation drop; arena-backed locals must not be dropped\n%s", out)
	}
	// …single destroy of the handle.
	if cnt := strings.Count(out, "destroy_arena arena"); cnt != 1 {
		t.Fatalf("expected exactly one destroy_arena arena; got %d\n%s", cnt, out)
	}
	// destroy before ret
	iDestroy := strings.Index(out, "destroy_arena arena")
	iRet := strings.Index(out, "ret")
	if !(iDestroy != -1 && iRet != -1 && iDestroy < iRet) {
		t.Fatalf("expected 'destroy_arena arena' before 'ret'\n%s", out)
	}
}
