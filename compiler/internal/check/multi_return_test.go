package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM4_MultiReturn_GroupedAssign_OK(t *testing.T) {
	// let a:int=0; let b:int=0; a, b := 1, 2
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.LetStmt{Name: ast.Ident{Name: "a"}, Type: &ast.TypeName{Name: "int"}, Value: &ast.IntLit{}},
			&ast.LetStmt{Name: ast.Ident{Name: "b"}, Type: &ast.TypeName{Name: "int"}, Value: &ast.IntLit{}},
			&ast.AssignStmt{
				LHS: []ast.Expr{&ast.Ident{Name: "a"}, &ast.Ident{Name: "b"}},
				RHS: []ast.Expr{&ast.IntLit{}, &ast.IntLit{}},
			},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

func TestM4_MultiReturn_GroupedAssign_ArityMismatch(t *testing.T) {
	// a, b := 1
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.LetStmt{Name: ast.Ident{Name: "a"}, Type: &ast.TypeName{Name: "int"}, Value: &ast.IntLit{}},
			&ast.LetStmt{Name: ast.Ident{Name: "b"}, Type: &ast.TypeName{Name: "int"}, Value: &ast.IntLit{}},
			&ast.AssignStmt{
				LHS: []ast.Expr{&ast.Ident{Name: "a"}, &ast.Ident{Name: "b"}},
				RHS: []ast.Expr{&ast.IntLit{}}, // fewer RHS than LHS
			},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "arity")
}

func TestM4_MultiReturn_GroupedAssign_TypeMismatch(t *testing.T) {
	// a:int, b:int := 1, "s"
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.LetStmt{Name: ast.Ident{Name: "a"}, Type: &ast.TypeName{Name: "int"}, Value: &ast.IntLit{}},
			&ast.LetStmt{Name: ast.Ident{Name: "b"}, Type: &ast.TypeName{Name: "int"}, Value: &ast.IntLit{}},
			&ast.AssignStmt{
				LHS: []ast.Expr{&ast.Ident{Name: "a"}, &ast.Ident{Name: "b"}},
				RHS: []ast.Expr{&ast.IntLit{}, &ast.StrLit{}},
			},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "cannot assign")
}
