package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

func TestM4_Comprehensions_List_Types(t *testing.T) {
	// let a = [x for ...] where elem is int → list[int]
	comp := &ast.ListComp{
		Elem: &ast.IntLit{},
	}
	let := &ast.LetStmt{Name: ast.Ident{Name: "a"}, Value: comp}
	main := &ast.FuncDecl{Name: ast.Ident{Name: "main"}, Body: &ast.Block{Stmts: []ast.Stmt{let}}}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, info := Check(mod)
	mustNoDiags(t, diags)

	got := info.Types[comp]
	want := types.ListOf(types.Int)
	if !types.Equal(got, want) {
		t.Fatalf("list comp type mismatch: got %v, want %v", got, want)
	}
}

func TestM4_Comprehensions_Set_Types(t *testing.T) {
	// let s = {x for ...} with elem str → set[str]
	comp := &ast.SetComp{
		Elem: &ast.StrLit{},
	}
	let := &ast.LetStmt{Name: ast.Ident{Name: "s"}, Value: comp}
	main := &ast.FuncDecl{Name: ast.Ident{Name: "main"}, Body: &ast.Block{Stmts: []ast.Stmt{let}}}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, info := Check(mod)
	mustNoDiags(t, diags)

	got := info.Types[comp]
	want := types.SetOf(types.Str)
	if !types.Equal(got, want) {
		t.Fatalf("set comp type mismatch: got %v, want %v", got, want)
	}
}

func TestM4_Comprehensions_Dict_Types(t *testing.T) {
	// let d = {k:v for ...} with k=int, v=str → dict[int,str]
	comp := &ast.DictComp{
		Key: &ast.IntLit{},
		Val: &ast.StrLit{},
	}
	let := &ast.LetStmt{Name: ast.Ident{Name: "d"}, Value: comp}
	main := &ast.FuncDecl{Name: ast.Ident{Name: "main"}, Body: &ast.Block{Stmts: []ast.Stmt{let}}}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, info := Check(mod)
	mustNoDiags(t, diags)

	got := info.Types[comp]
	want := types.DictOf(types.Int, types.Str)
	if !types.Equal(got, want) {
		t.Fatalf("dict comp type mismatch: got %v, want %v", got, want)
	}
}
