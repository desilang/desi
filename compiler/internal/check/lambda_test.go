package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// In M4 we require typed lambda params.
// We just ensure a let-binding of a typed lambda produces no diagnostics.
// (Calling the lambda via an identifier variable would require variable-call support;
// we can add that in a later phase.)
func TestM4_Lambda_TypedParam_BindsOK(t *testing.T) {
	lam := &ast.LambdaExpr{
		Params: []ast.Param{
			{Name: ast.Ident{Name: "n"}, Type: &ast.TypeName{Name: "int"}},
		},
		Body: &ast.IntLit{}, // returns int; body doesn't need to reference n in M4
	}
	let := &ast.LetStmt{Name: ast.Ident{Name: "x"}, Value: lam}

	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{let}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}
