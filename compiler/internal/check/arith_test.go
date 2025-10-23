package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// Build: fn ok() { 1 + 2;  1.0 + 2.0; }
func TestM4_Arith_Ok(t *testing.T) {
	fn := &ast.FuncDecl{
		Name:    ast.Ident{Name: "ok"},
		Params:  nil,
		RetType: nil,
		Body: &ast.Block{
			Stmts: []ast.Stmt{
				&ast.ExprStmt{Expr: &ast.BinaryExpr{Op: "+", Lhs: &ast.IntLit{}, Rhs: &ast.IntLit{}}},
				&ast.ExprStmt{Expr: &ast.BinaryExpr{Op: "+", Lhs: &ast.FloatLit{}, Rhs: &ast.FloatLit{}}},
			},
		},
	}
	mod := &ast.Module{Filename: "<mem>", Decls: []ast.Decl{fn}}
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

// Build: fn bad() { 1 + true }
func TestM4_Arith_BadMix(t *testing.T) {
	fn := &ast.FuncDecl{
		Name:    ast.Ident{Name: "bad"},
		RetType: nil,
		Body: &ast.Block{
			Stmts: []ast.Stmt{
				&ast.ExprStmt{Expr: &ast.BinaryExpr{Op: "+", Lhs: &ast.IntLit{}, Rhs: &ast.BoolLit{}}},
			},
		},
	}
	mod := &ast.Module{Filename: "<mem>", Decls: []ast.Decl{fn}}
	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "invalid operand")
}
