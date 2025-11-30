package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

func TestM8E_Types_AsyncCallAndAwait(t *testing.T) {
	// async def add1(x:int) -> int: return x + 1
	add1 := &ast.FuncDecl{
		Async:   true,
		Name:    ast.Ident{Name: "add1"},
		Params:  []ast.Param{{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "int"}}},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.ReturnStmt{Value: &ast.BinaryExpr{
				Op:  "+",
				Lhs: &ast.Ident{Name: "x"},
				Rhs: &ast.IntLit{Text: "1"},
			}},
		}},
	}

	// def main() -> int:
	//   fut = add1(41)
	//   n = await fut
	//   0
	call := &ast.CallExpr{Callee: &ast.Ident{Name: "add1"}, Args: []ast.Expr{&ast.IntLit{Text: "41"}}}
	aw := &ast.UnaryExpr{Op: "await", X: &ast.Ident{Name: "fut"}}
	main := &ast.FuncDecl{
		Name:    ast.Ident{Name: "main"},
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.LetStmt{Name: ast.Ident{Name: "fut"}, Type: &ast.TypeName{Name: "future", Params: []*ast.TypeName{{Name: "int"}}}, Value: call},
			&ast.LetStmt{Name: ast.Ident{Name: "n"}, Type: &ast.TypeName{Name: "int"}, Value: aw},
			&ast.ReturnStmt{Value: &ast.IntLit{Text: "0"}},
		}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{add1, main}}
	diags, info := Check(mod)
	mustNoDiags(t, diags)

	// call type: future[int]
	if got := info.Types[call]; got == nil || got.String() != "future[int]" {
		t.Fatalf("call type = %v, want future[int]", got)
	}
	// await type: int
	if got := info.Types[aw]; got == nil || !types.Equal(got, types.Int) {
		t.Fatalf("await type = %v, want int", got)
	}
}
