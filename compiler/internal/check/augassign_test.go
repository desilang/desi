package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM4_AugAssign_IntOK(t *testing.T) {
	// let x:int = 1; x += 2
	let := &ast.LetStmt{
		Name:  ast.Ident{Name: "x"},
		Type:  &ast.TypeName{Name: "int"},
		Value: &ast.IntLit{},
	}
	aug := &ast.AugAssignStmt{
		Op:    "+=",
		Left:  &ast.Ident{Name: "x"},
		Right: &ast.IntLit{},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{let, aug}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

func TestM4_AugAssign_TypeMismatch(t *testing.T) {
	// let x:int = 1; x += 1.0  -> invalid augmented assignment
	let := &ast.LetStmt{
		Name:  ast.Ident{Name: "x"},
		Type:  &ast.TypeName{Name: "int"},
		Value: &ast.IntLit{},
	}
	aug := &ast.AugAssignStmt{
		Op:    "+=",
		Left:  &ast.Ident{Name: "x"},
		Right: &ast.FloatLit{},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{let, aug}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "augmented")
}
