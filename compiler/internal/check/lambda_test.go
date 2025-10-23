package check

import (
  "testing"

  "github.com/desilang/desi/compiler/internal/ast"
)

// In M4 we require typed lambda params.
// Ensure a let-binding of a typed lambda produces no diagnostics.
func TestM4_Lambda_TypedParam_BindsOK(t *testing.T) {
  lam := &ast.LambdaExpr{
    Params: []ast.LambdaParam{
      {Name: ast.Ident{Name: "n"}, Type: &ast.TypeName{Name: "int"}},
    },
    Body: &ast.IntLit{}, // returns int
  }
  let := &ast.LetStmt{
    Name:  ast.Ident{Name: "x"},
    Value: lam,
  }

  main := &ast.FuncDecl{
    Name: ast.Ident{Name: "main"},
    Body: &ast.Block{Stmts: []ast.Stmt{let}},
  }
  mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}
  diags, _ := Check(mod)
  mustNoDiags(t, diags)
}
