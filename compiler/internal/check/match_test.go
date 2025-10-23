package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM4_Match_ArmsSameType_OK(t *testing.T) {
	// match x: case A -> 1; case B -> 2
	m := &ast.MatchStmt{
		Scrutinee: &ast.Ident{Name: "x"},
		Arms: []ast.MatchArm{
			{Pattern: &ast.Ident{Name: "A"}, Result: &ast.IntLit{}},
			{Pattern: &ast.Ident{Name: "B"}, Result: &ast.IntLit{}},
		},
	}
	main := &ast.FuncDecl{Name: ast.Ident{Name: "main"}, Body: &ast.Block{Stmts: []ast.Stmt{m}}}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

func TestM4_Match_ArmsTypeMismatch(t *testing.T) {
	// match x: case A -> 1; case B -> "s"
	m := &ast.MatchStmt{
		Scrutinee: &ast.Ident{Name: "x"},
		Arms: []ast.MatchArm{
			{Pattern: &ast.Ident{Name: "A"}, Result: &ast.IntLit{}},
			{Pattern: &ast.Ident{Name: "B"}, Result: &ast.StrLit{}},
		},
	}
	main := &ast.FuncDecl{Name: ast.Ident{Name: "main"}, Body: &ast.Block{Stmts: []ast.Stmt{m}}}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "match arm")
}
