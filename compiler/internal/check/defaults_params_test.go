package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

// NOTE: These tests are scaffolding for M14 default-parameter rules.
// The checker does not yet emit DDF0001–DDF0004, so we skip them for now.
// When you implement the rules, delete the t.Skip(...) calls and make sure
// the assertions line up with the actual messages/codes.

func TestM14_Defaults_MustBeTrailing(t *testing.T) {
	t.Skip("defaults semantics (DDF0001) not implemented yet")

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

	mustHaveSomeDiagContaining(t, diags, "parameters with defaults must come last")
}

func TestM14_Defaults_RequireAnnotation(t *testing.T) {
	t.Skip("defaults semantics (DDF0002) not implemented yet")

	// def f(x = 1): ...
	f := &ast.FuncDecl{
		Name: ast.Ident{Name: "f"},
		Params: []ast.Param{
			{
				Name:    ast.Ident{Name: "x"},
				Type:    nil, // no annotation
				Default: &ast.IntLit{},
			},
		},
		// Return type intentionally omitted.
		Body: &ast.Block{},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f}}
	diags, _ := Check(mod)

	mustHaveSomeDiagContaining(t, diags, "defaulted parameter must have a type annotation")
}

func TestM14_Defaults_TypeMismatch(t *testing.T) {
	t.Skip("defaults semantics (DDF0003) not implemented yet")

	// def f(x: int = 1.0): ...
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

	mustHaveSomeDiagContaining(t, diags, "default value incompatible with parameter type")
}

func TestM14_Defaults_OverloadConsistency(t *testing.T) {
	t.Skip("defaults semantics (DDF0004) not implemented yet")

	// Overloads disagree on which param has a default:
	//
	//   def f(x: int, y: int = 1)
	//   def f(x: int, y: int)
	f1 := &ast.FuncDecl{
		Name: ast.Ident{Name: "f"},
		Params: []ast.Param{
			{
				Name: ast.Ident{Name: "x"},
				Type: &ast.TypeName{Name: "int"},
			},
			{
				Name:    ast.Ident{Name: "y"},
				Type:    &ast.TypeName{Name: "int"},
				Default: &ast.IntLit{},
			},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	f2 := &ast.FuncDecl{
		Name: ast.Ident{Name: "f"},
		Params: []ast.Param{
			{
				Name: ast.Ident{Name: "x"},
				Type: &ast.TypeName{Name: "int"},
			},
			{
				Name: ast.Ident{Name: "y"},
				Type: &ast.TypeName{Name: "int"},
			},
		},
		RetType: &ast.TypeName{Name: "int"},
		Body:    &ast.Block{},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{f1, f2}}
	diags, _ := Check(mod)

	mustHaveSomeDiagContaining(t, diags, "default parameters must be consistent across overloads")
}
