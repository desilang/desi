package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/types"
)

func makeExternSin() *ast.FuncDecl {
	// @extern pub def sin(x: float) -> float
	// Args to @extern are irrelevant for this test; checker only needs to know it's extern.
	return &ast.FuncDecl{
		Pub:  true,
		Name: ast.Ident{Name: "sin"},
		Decorators: []*ast.Decorator{
			{
				Name: ast.Ident{Name: "extern"},
				// Args intentionally omitted; shape validation isn't under test here.
			},
		},
		Params:  []ast.Param{{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: "float"}}},
		RetType: &ast.TypeName{Name: "float"},
		Body:    nil, // prototype-only extern
	}
}

func TestFFI_Call_SafeContext_DFI0003(t *testing.T) {
	// def main() -> int:
	//   let y = sin(0.5)   # should error (DFI0003)
	//   0
	call := &ast.CallExpr{Callee: &ast.Ident{Name: "sin"}, Args: []ast.Expr{&ast.FloatLit{Text: "0.5"}}}
	main := &ast.FuncDecl{
		Name:    ast.Ident{Name: "main"},
		Params:  nil,
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.LetStmt{Name: ast.Ident{Name: "y"}, Type: &ast.TypeName{Name: "float"}, Value: call},
			&ast.ReturnStmt{Value: &ast.IntLit{Text: "0"}},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{makeExternSin(), main}}

	diags, _ := Check(mod)
	expectAnyDiag(t, diags, "DFI0003")
}

func TestFFI_Call_UnsafeBlock_OK(t *testing.T) {
	// def main() -> int:
	//   unsafe:
	//     let y = sin(0.5)   # OK
	//   0
	call := &ast.CallExpr{Callee: &ast.Ident{Name: "sin"}, Args: []ast.Expr{&ast.FloatLit{Text: "0.5"}}}
	unsafeBlk := &ast.UnsafeBlock{
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.LetStmt{Name: ast.Ident{Name: "y"}, Type: &ast.TypeName{Name: "float"}, Value: call},
		}},
	}
	main := &ast.FuncDecl{
		Name:    ast.Ident{Name: "main"},
		Params:  nil,
		RetType: &ast.TypeName{Name: "int"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			unsafeBlk,
			&ast.ReturnStmt{Value: &ast.IntLit{Text: "0"}},
		}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{makeExternSin(), main}}

	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

// tiny helper: assert a particular code id appears
func expectAnyDiag(t *testing.T, diags []diag.Diagnostic, needle string) {
	t.Helper()
	for _, d := range diags {
		if d.CodeID == needle {
			return
		}
	}
	t.Fatalf("expected diagnostic %s, got none", needle)
}

// ensure types to avoid unused warnings (imported)
var _ = types.Int
