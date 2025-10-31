package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM6P2_UseAfterMove_LocalCall(t *testing.T) {
	// def take(y:int) -> none: return none
	take := &ast.FuncDecl{
		Name: ast.Ident{Name: "take"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "y"}, Type: &ast.TypeName{Name: "int"}},
		},
		RetType: &ast.TypeName{Name: "none"},
		Body:    &ast.Block{},
	}

	// def main(): let t = 1; take(t); t
	let := &ast.LetStmt{
		Name:  ast.Ident{Name: "t"},
		Value: &ast.IntLit{Text: "1"},
	}
	call := &ast.ExprStmt{
		Expr: &ast.CallExpr{
			Callee: &ast.Ident{Name: "take"},
			Args:   []ast.Expr{&ast.Ident{Name: "t"}},
		},
	}
	use := &ast.ExprStmt{Expr: &ast.Ident{Name: "t"}}

	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{let, call, use}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{take, main}}

	diags, _ := Check(mod)
	mustHaveSomeDiagContaining(t, diags, "moved earlier")
}
