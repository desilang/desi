package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM4_Comprehension_List_AssignToInt_Error(t *testing.T) {
	// let x:int = [1 for ...]  => list[int] not assignable to int
	comp := &ast.ListComp{Elem: &ast.IntLit{}}
	let := &ast.LetStmt{
		Name:  ast.Ident{Name: "x"},
		Type:  &ast.TypeName{Name: "int"},
		Value: comp,
	}
	main := &ast.FuncDecl{Name: ast.Ident{Name: "main"}, Body: &ast.Block{Stmts: []ast.Stmt{let}}}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "cannot assign")
}

func TestM4_Comprehension_Dict_AssignToInt_Error(t *testing.T) {
	// let y:int = { "k": 1 for ... } => dict[str,int] not assignable to int
	comp := &ast.DictComp{Key: &ast.StrLit{}, Val: &ast.IntLit{}}
	let := &ast.LetStmt{
		Name:  ast.Ident{Name: "y"},
		Type:  &ast.TypeName{Name: "int"},
		Value: comp,
	}
	main := &ast.FuncDecl{Name: ast.Ident{Name: "main"}, Body: &ast.Block{Stmts: []ast.Stmt{let}}}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "cannot assign")
}
