package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestBaseLvalue_Ident_IsLvalue(t *testing.T) {
	c := &checker{}
	name, ok := c.baseLvalue(&ast.Ident{Name: "x"})
	if !ok || name != "x" {
		t.Fatalf("ident should be lvalue base 'x'; got name=%q ok=%v", name, ok)
	}
}

func TestBaseLvalue_Field_FromIdent_IsLvalue(t *testing.T) {
	c := &checker{}
	f := &ast.FieldExpr{
		X:    &ast.Ident{Name: "p"},
		Name: ast.Ident{Name: "f"},
	}
	name, ok := c.baseLvalue(f)
	if !ok || name != "p" {
		t.Fatalf("p.f should be lvalue base 'p'; got name=%q ok=%v", name, ok)
	}
}

func TestBaseLvalue_Index_FromIdent_IsLvalue(t *testing.T) {
	c := &checker{}
	idx := &ast.IndexExpr{
		X:   &ast.Ident{Name: "a"},
		Idx: &ast.IntLit{},
	}
	name, ok := c.baseLvalue(idx)
	if !ok || name != "a" {
		t.Fatalf("a[i] should be lvalue base 'a'; got name=%q ok=%v", name, ok)
	}
}

func TestBaseLvalue_Field_FromRvalue_NotLvalue(t *testing.T) {
	c := &checker{}
	// (a+b).f  → base is a BinaryExpr (rvalue) → not an lvalue
	f := &ast.FieldExpr{
		X: &ast.BinaryExpr{
			Op:  "+",
			Lhs: &ast.Ident{Name: "a"},
			Rhs: &ast.Ident{Name: "b"},
		},
		Name: ast.Ident{Name: "f"},
	}
	if name, ok := c.baseLvalue(f); ok {
		t.Fatalf("(a+b).f should NOT be an lvalue; got base=%q ok=%v", name, ok)
	}
}

func TestBaseLvalue_Index_FromCall_NotLvalue(t *testing.T) {
	c := &checker{}
	// foo().i  → base is a CallExpr (rvalue) → not an lvalue
	idx := &ast.IndexExpr{
		X: &ast.CallExpr{
			Callee: &ast.Ident{Name: "foo"},
			Args:   nil,
		},
		Idx: &ast.IntLit{},
	}
	if name, ok := c.baseLvalue(idx); ok {
		t.Fatalf("call()[i] should NOT be an lvalue; got base=%q ok=%v", name, ok)
	}
}
