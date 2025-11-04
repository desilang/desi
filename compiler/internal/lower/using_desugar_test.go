package lower_test

import (
  "bytes"
  "strings"
  "testing"

  "github.com/desilang/desi/compiler/internal/ast"
  "github.com/desilang/desi/compiler/internal/hir"
  "github.com/desilang/desi/compiler/internal/lower"
)

func TestLower_UsingDesugar_DestroyRunsOnEarlyReturn(t *testing.T) {
  // using arena:
  //   let x = arena.alloc(1)
  //   if true: return
  using := &ast.UsingStmt{
    Bind: &ast.Ident{Name: "arena"},
    Body: &ast.Block{Stmts: []ast.Stmt{
      &ast.LetStmt{
        Name: ast.Ident{Name: "x"},
        Value: &ast.CallExpr{
          Callee: &ast.FieldExpr{
            X:    &ast.Ident{Name: "arena"},
            Name: ast.Ident{Name: "alloc"},
          },
          Args: []ast.Expr{&ast.IntLit{Text: "1"}},
        },
      },
      &ast.IfStmt{
        Cond: &ast.BoolLit{Value: true},
        Then: &ast.Block{Stmts: []ast.Stmt{&ast.ReturnStmt{}}},
      },
    }},
  }
  blk := &ast.Block{Stmts: []ast.Stmt{using}}
  fn := lower.LowerBlock("test", blk)

  var buf bytes.Buffer
  hir.Print(&buf, fn)
  out := buf.String()

  countDestroy := strings.Count(out, "drop arena")
  if countDestroy != 1 {
    t.Fatalf("expected exactly one destroy (drop arena); got %d\n%s", countDestroy, out)
  }
  // it must appear before ret
  iDrop := strings.Index(out, "drop arena")
  iRet := strings.Index(out, "ret")
  if !(iDrop != -1 && iRet != -1 && iDrop < iRet) {
    t.Fatalf("expected 'drop arena' before 'ret'\n%s", out)
  }
}
