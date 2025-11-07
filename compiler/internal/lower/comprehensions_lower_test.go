package lower

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/hir"
)

func TestLower_ListComp_EmitsListPush(t *testing.T) {
	// ys = [1 for x in xs if true]
	comp := &ast.ListComp{
		Elem: &ast.IntLit{Text: "1"},
		// We don't need clauses for Tier-0 shape; elem is enough to test the append.
	}
	let := &ast.LetStmt{
		Name:  ast.Ident{Name: "ys"},
		Value: comp,
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{let}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	fn := LowerBlockFromSource("main", main.Body, nil)
	out := hir.Print(fn)

	if !strings.Contains(out, "let ys") {
		t.Fatalf("HIR missing 'let ys' binding:\n%s", out)
	}
	if !strings.Contains(out, "call list_push(") {
		t.Fatalf("HIR missing list_push append for list comprehension:\n%s", out)
	}
}
