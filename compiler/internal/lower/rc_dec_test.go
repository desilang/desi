package lower_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/hir"
	"github.com/desilang/desi/compiler/internal/lower"
	"github.com/desilang/desi/compiler/internal/types"
)

func TestLower_Rc_DecrefAtScopeEnd(t *testing.T) {
	let := &ast.LetStmt{Name: ast.Ident{Name: "x"}, Value: &ast.IntLit{Text: "1"}}
	blk := &ast.Block{
		Stmts: []ast.Stmt{
			let,
			&ast.ReturnStmt{},
		},
	}
	info := &check.Info{
		Idents: map[*ast.Ident]*check.Symbol{
			&let.Name: {Name: "x", Kind: check.SymVar, Type: types.RcOf(types.Int)},
		},
	}
	fn := lower.LowerBlockWithInfo("test", blk, info)

	var buf bytes.Buffer
	hir.Print(&buf, fn)
	out := buf.String()

	if strings.Contains(out, "drop x") {
		t.Fatalf("unexpected raw drop for rc-var x:\n%s", out)
	}
	if !strings.Contains(out, "decref x") {
		t.Fatalf("missing decref for rc-var x:\n%s", out)
	}
	// exactly one decref
	if cnt := strings.Count(out, "decref x"); cnt != 1 {
		t.Fatalf("expected exactly one decref x, got %d\n%s", cnt, out)
	}
}
