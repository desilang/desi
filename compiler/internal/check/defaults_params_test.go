package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// These tests cover the M14 declaration-side default-parameter rules that are
// implemented today:
//   - defaults must be trailing
//   - inout params cannot have defaults
//   - defaults must be compile-time constants
//   - if a param is annotated, the default's type must match

func TestM14_Defaults_MustBeTrailing(t *testing.T) {
	// def f(a: int = 1, b: int): ...
	f := &ast.FuncDecl{
		Name: ast.Ident{Name: "f"},
		Params: []ast.Param{
			{
				Name:    ast.Ident{Name: "a"},
				Type:    &ast.TypeName{Name: "int"},
				Default: &ast.IntLit{},
			},
			{
				Name: ast.Ident{Name: "b"},
				Type: &ast.TypeName{Name: "int"},
			},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)

	mustHaveSomeDiagContaining(t, diags, "non-default parameter cannot follow parameter with default")
}

func TestM14_Defaults_InoutForbidden(t *testing.T) {
	// def f(inout a: int = 1): ...
	f := &ast.FuncDecl{
		Name: ast.Ident{Name: "f"},
		Params: []ast.Param{
			{
				Name:    ast.Ident{Name: "a"},
				Type:    &ast.TypeName{Name: "int"},
				Mode:    ast.ParamInout,
				Default: &ast.IntLit{},
			},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)

	mustHaveSomeDiagContaining(t, diags, "inout parameters cannot have default values")
}

func TestM14_Defaults_MustBeConst(t *testing.T) {
	// def f(x: int = 1 + 2): ...  // non-const expression => reject
	f := &ast.FuncDecl{
		Name: ast.Ident{Name: "f"},
		Params: []ast.Param{
			{
				Name: ast.Ident{Name: "x"},
				Type: &ast.TypeName{Name: "int"},
				Default: &ast.BinaryExpr{
					Op:  "+",
					Lhs: &ast.IntLit{},
					Rhs: &ast.IntLit{},
				},
			},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)

	mustHaveSomeDiagContaining(t, diags, "default value must be a compile-time constant")
}

func TestM14_Defaults_TypeMismatch(t *testing.T) {
	// def f(x: int = 1.0): ...  // default is float, param is int => type mismatch
	f := &ast.FuncDecl{
		Name: ast.Ident{Name: "f"},
		Params: []ast.Param{
			{
				Name:    ast.Ident{Name: "x"},
				Type:    &ast.TypeName{Name: "int"},
				Default: &ast.FloatLit{},
			},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)

	// validateParamDefaults currently reports this via DTE0004 with a message like:
	// "default value has type float, expected int"
	mustHaveSomeDiagContaining(t, diags, "default value has type")
}
