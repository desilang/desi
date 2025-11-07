package lower

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
)

func TestLower_ListComp_EmitsListPush(t *testing.T) {
	// ys = [1 for ...]  (we only need Elem to exercise the append path)
	comp := &ast.ListComp{
		Elem: &ast.IntLit{Text: "1"},
	}
	let := &ast.LetStmt{
		Name:  ast.Ident{Name: "ys"},
		Value: comp,
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{let}},
	}

	fn := LowerBlock("main", main.Body)

	var buf bytes.Buffer
	hir.Print(&buf, fn)
	out := buf.String()

	if !strings.Contains(out, "call list_push(") {
		t.Fatalf("HIR missing list_push append for list comprehension:\n%s", out)
	}
}
