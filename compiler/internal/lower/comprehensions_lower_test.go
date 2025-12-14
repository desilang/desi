package lower

import (
	"bytes"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
)

func TestLower_ListComp_EmitsListAppend(t *testing.T) {
	// ys = [1 for x in xs] - with proper clause structure
	xvar := &ast.Ident{Name: "x"}
	comp := &ast.ListComp{
		Elem: &ast.IntLit{Text: "1"},
		Clauses: []ast.CompClause{{
			Target: xvar,
			Iter:   &ast.Ident{Name: "xs"},
			If:     nil,
		}},
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

	if !strings.Contains(out, "list_append") {
		t.Fatalf("HIR missing list_append for list comprehension:\n%s", out)
	}
}
