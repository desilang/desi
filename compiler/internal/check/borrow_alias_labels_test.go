package check

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM6_Aliasing_Adds_Secondary_Label(t *testing.T) {
	// pub def g(inout a:int, ref b:int) -> none
	g := &ast.FuncDecl{
		Pub:  true,
		Name: ast.Ident{Name: "g"},
		Params: []ast.Param{
			{Name: ast.Ident{Name: "a"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamInout},
			{Name: ast.Ident{Name: "b"}, Type: &ast.TypeName{Name: "int"}, Mode: ast.ParamRef},
		},
		RetType: &ast.TypeName{Name: "none"},
	}

	// def main(): let x=0; g(x, x)  // aliasing between a and b
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{
			&ast.LetStmt{Name: ast.Ident{Name: "x"}, Value: &ast.IntLit{}},
			&ast.ExprStmt{Expr: &ast.CallExpr{
				Callee: &ast.Ident{Name: "g"},
				Args:   []ast.Expr{&ast.Ident{Name: "x"}, &ast.Ident{Name: "x"}},
			}},
		}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{g, main}}
	diags, _ := Check(mod)

	// Find an alias diagnostic and ensure it has a non-primary label.
	found := false
	for _, d := range diags {
		if strings.Contains(strings.ToLower(d.Message), "alias") {
			// require at least one non-primary label
			for _, lab := range d.Labels {
				if !lab.Primary {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
	}
	if !found {
		t.Fatalf("expected alias diagnostic to include a secondary label pointing to the other argument site")
	}
}
