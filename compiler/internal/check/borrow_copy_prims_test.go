package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// buildCopyPrimModule builds:
//
//	def take(x:<T>) -> none: pass
//	def main(): let t=<lit>; take(t); t
func buildCopyPrimModule(paramType string, lit ast.Expr) *ast.Module {
	take := &ast.FuncDecl{
		Name: ast.Ident{Name: "take"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "x"}, Type: &ast.TypeName{Name: paramType}},
		},
		RetType: &ast.TypeName{Name: "none"},
		Body:    &ast.Block{},
	}
	let := &ast.LetStmt{
		Name:  ast.Ident{Name: "t"},
		Value: lit,
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
	return &ast.Module{File: "<mem>", Decls: []ast.Decl{take, main}}
}

func TestM6_Copy_Prim_Int_NoUseAfterMove(t *testing.T) {
	mod := buildCopyPrimModule("int", &ast.IntLit{Text: "1"})
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

func TestM6_Copy_Prim_Float_NoUseAfterMove(t *testing.T) {
	mod := buildCopyPrimModule("float", &ast.FloatLit{Text: "1.0"})
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

func TestM6_Copy_Prim_Bool_NoUseAfterMove(t *testing.T) {
	mod := buildCopyPrimModule("bool", &ast.BoolLit{Value: true})
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}

func TestM6_Copy_Prim_Str_NoUseAfterMove(t *testing.T) {
	// StrLit carries only (Long, Span); presence is enough for typing to str.
	mod := buildCopyPrimModule("str", &ast.StrLit{})
	diags, _ := Check(mod)
	mustNoDiags(t, diags)
}
