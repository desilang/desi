package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM6_Ref_Requires_Lvalue_RvalueFails(t *testing.T) {
	// def take(ref x:int) -> none
	take := &ast.FuncDecl{
		Name: ast.Ident{Name: "take"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamRef},
		},
		RetType: &ast.TypeName{Name: "none"},
	}

	// def main(): take(1)
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ExprStmt{Expr: &ast.CallExpr{
				Callee: &ast.Ident{Name: "take"},
				Args:   []ast.Expr{&ast.IntLit{}}, // rvalue ⇒ should fail for ref
			}},
		}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{take, main}}
	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "lvalue")
}

func TestM6_Ref_Requires_Lvalue_IdentOK(t *testing.T) {
	// def take(ref x:int) -> none
	take := &ast.FuncDecl{
		Name: ast.Ident{Name: "take"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamRef},
		},
		RetType: &ast.TypeName{Name: "none"},
	}

	// def main(): let a=0; take(a)
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.LetStmt{Name: ast.Ident{Name: "a"}, Type: &ast.TypeName{Name: "int"}, Value: &ast.IntLit{}},
			&ast.ExprStmt{Expr: &ast.CallExpr{
				Callee: &ast.Ident{Name: "take"},
				Args:   []ast.Expr{&ast.Ident{Name: "a"}}, // lvalue ⇒ OK
			}},
		}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{take, main}}
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}
