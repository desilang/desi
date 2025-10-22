package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func stringOrNil(t any) string {
	if t == nil {
		return "<nil>"
	}
	type s interface{ String() string }
	if v, ok := t.(s); ok {
		return v.String()
	}
	return "<no String()>"
}

func TestM4_Comprehension_List_Types(t *testing.T) {
	lc := &ast.ListComp{Elem: &ast.IntLit{}}
	fn := &ast.FuncDecl{
		Name: ast.Ident{Name: "f"},
		Body: &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: lc}}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{fn}}
	diags, info := Check(mod)
	mustNoDiags(t, diags)

	got := info.Types[lc]
	if got == nil || got.String() != "list[int]" {
		t.Fatalf("want list[int], got %s", stringOrNil(got))
	}
}

func TestM4_Comprehension_Dict_Types(t *testing.T) {
	dc := &ast.DictComp{Key: &ast.StrLit{}, Val: &ast.IntLit{}}
	fn := &ast.FuncDecl{
		Name: ast.Ident{Name: "g"},
		Body: &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: dc}}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{fn}}
	diags, info := Check(mod)
	mustNoDiags(t, diags)

	got := info.Types[dc]
	if got == nil || got.String() != "dict[str,int]" {
		t.Fatalf("want dict[str,int], got %s", stringOrNil(got))
	}
}
