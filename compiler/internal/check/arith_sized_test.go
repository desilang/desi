package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// helper to build a function with two let declarations and one binary expr using them.
func modWithBinary(t1, t2, op string) *ast.Module {
	letA := &ast.LetStmt{
		Mutable: false,
		Name:    ast.Ident{Name: "a"},
		Type:    &ast.TypeName{Name: t1},
		// no initializer: type is as-declared
	}
	letB := &ast.LetStmt{
		Mutable: false,
		Name:    ast.Ident{Name: "b"},
		Type:    &ast.TypeName{Name: t2},
	}
	ex := &ast.ExprStmt{Expr: &ast.BinaryExpr{
		Op:  op,
		Lhs: &ast.Ident{Name: "a"},
		Rhs: &ast.Ident{Name: "b"},
	}}
	fn := &ast.FuncDecl{
		Name:    ast.Ident{Name: "f"},
		Params:  nil,
		RetType: nil,
		Body:    &ast.Block{Stmts: []ast.Stmt{letA, letB, ex}},
	}
	return &ast.Module{File: "<mem>", Decls: []ast.Decl{fn}}
}

func hasCode(diags []diag.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.CodeID == code {
			return true
		}
	}
	return false
}

func TestArithSized_OK_IntsAndFloats(t *testing.T) {
	// u8 + u8
	diags1, _ := Check(modWithBinary("u8", "u8", "+"))
	mustNoDiags(t, diags1)

	// i16 * i16
	diags2, _ := Check(modWithBinary("i16", "i16", "*"))
	mustNoDiags(t, diags2)

	// f32 + f32
	diags3, _ := Check(modWithBinary("f32", "f32", "+"))
	mustNoDiags(t, diags3)

	// f64 / f64 (float is alias of f64)
	diags4, _ := Check(modWithBinary("f64", "f64", "/"))
	mustNoDiags(t, diags4)
}

func TestArithSized_Errors(t *testing.T) {
	// signedness mismatch: u8 + i8 -> DNT0002
	diags1, _ := Check(modWithBinary("u8", "i8", "+"))
	if !hasCode(diags1, "DNT0002") {
		t.Fatalf("expected DNT0002, got: %+v", diags1)
	}

	// width mismatch: u8 + u16 -> DNT0001
	diags2, _ := Check(modWithBinary("u8", "u16", "+"))
	if !hasCode(diags2, "DNT0001") {
		t.Fatalf("expected DNT0001, got: %+v", diags2)
	}

	// width mismatch floats: f32 + f64 -> DNT0001
	diags3, _ := Check(modWithBinary("f32", "f64", "+"))
	if !hasCode(diags3, "DNT0001") {
		t.Fatalf("expected DNT0001, got: %+v", diags3)
	}
}
