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

func TestLower_Rc_Move_NoExtraInc_OneDecAtFinalOwner(t *testing.T) {
	letX := &ast.LetStmt{Name: ast.Ident{Name: "x"}, Value: &ast.IntLit{Text: "1"}}
	letY := &ast.LetStmt{Name: ast.Ident{Name: "y"}, Value: &ast.Ident{Name: "x"}}
	blk := &ast.Block{
		Stmts: []ast.Stmt{
			letX,
			letY,
			&ast.ReturnStmt{},
		},
	}
	rcInt := types.RcOf(types.Int)
	info := &check.Info{
		Idents: map[*ast.Ident]*check.Symbol{
			&letX.Name: {Name: "x", Kind: check.SymVar, Type: rcInt},
			&letY.Name: {Name: "y", Kind: check.SymVar, Type: rcInt},
		},
	}
	fn := lower.LowerBlockWithInfo("test", blk, info)

	var buf bytes.Buffer
	hir.Print(&buf, fn)
	out := buf.String()

	// moved-from 'x' should not decref at exit; only 'y' decref once
	if strings.Contains(out, "decref x") {
		t.Fatalf("unexpected decref of moved-from x:\n%s", out)
	}
	if cnt := strings.Count(out, "decref y"); cnt != 1 {
		t.Fatalf("expected exactly one decref y, got %d\n%s", cnt, out)
	}
	// and there must be no "incref" in M7B
	if strings.Contains(out, "incref") {
		t.Fatalf("unexpected incref in M7B:\n%s", out)
	}
}
