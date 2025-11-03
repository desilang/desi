package lower_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/lower"
)

func TestLower_DropsAtFuncExitReverseInit(t *testing.T) {
	blk := &ast.Block{
		Stmts: []ast.Stmt{
			&ast.LetStmt{Name: ast.Ident{Name: "a"}, Value: &ast.IntLit{Text: "1"}},
			&ast.LetStmt{Name: ast.Ident{Name: "b"}, Value: &ast.IntLit{Text: "2"}},
			&ast.ExprStmt{Expr: &ast.Ident{Name: "a"}}, // use a
			&ast.ReturnStmt{}, // early return
		},
	}
	fn := lower.LowerBlock("test", blk)
	var buf bytes.Buffer
	hir.Print(&buf, fn)
	out := buf.String()

	// Expect drops for b then a, before ret.
	ib := strings.Index(out, "drop b")
	ia := strings.Index(out, "drop a")
	ir := strings.Index(out, "ret")
	if ib == -1 || ia == -1 {
		t.Fatalf("expected drop b and drop a in output\n%s", out)
	}
	if !(ib < ia && ia < ir) {
		t.Fatalf("expected order: drop b < drop a < ret; got\n%s", out)
	}
}
