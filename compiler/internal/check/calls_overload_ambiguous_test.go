package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// Two identical f(int)->int overloads; call f(1) -> ambiguous
func TestM4_Calls_Overload_Ambiguous(t *testing.T) {
	f1 := &ast.FuncDecl{
		Name:    ast.Ident{Name: "f"},
		Params:  []ast.Param{{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}}},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	f2 := &ast.FuncDecl{
		Name:    ast.Ident{Name: "f"},
		Params:  []ast.Param{{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}}},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{
			Stmts: []ast.Stmt{
				&ast.ExprStmt{Expr: &ast.CallExpr{
					Callee: &ast.Ident{Name: "f"},
					Args:   []ast.Expr{&ast.IntLit{}},
				}},
			},
		},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f1, f2, main}}
	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "ambiguous overload")
}
