package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

func TestM9A_TypeString_USize_ISize_CPtr(t *testing.T) {
	if got, ok := types.FromName("usize"); !ok || got.String() != "usize" {
		t.Fatalf("usize FromName/String: ok=%v, got=%v", ok, got)
	}
	if got, ok := types.FromName("isize"); !ok || got.String() != "isize" {
		t.Fatalf("isize FromName/String: ok=%v, got=%v", ok, got)
	}
	cp := types.CPtrOf(types.Int)
	if cp.String() != "cptr[int]" {
		t.Fatalf("cptr[int] String() => %q", cp.String())
	}
}

func TestM9A_USize_Arith_Minimal(t *testing.T) {
	// Build:
	// let u: usize = 1
	// u + 2
	uIdent := ast.Ident{Name: "u"}
	bin := &ast.BinaryExpr{Op: "+", Lhs: &uIdent, Rhs: &ast.IntLit{}}
	fn := &ast.FuncDecl{
		Name: ast.Ident{Name: "f"},
		Body: &ast.Block{
			Stmts: []ast.Stmt{
				&ast.LetStmt{
					Name:  uIdent,
					Type:  &ast.TypeName{Name: "usize"},
					Value: &ast.IntLit{},
				},
				&ast.ExprStmt{Expr: bin},
			},
		},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{fn}}

	diags, info := Check(mod)
	if len(diags) != 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}
	typ := info.Types[bin]
	if typ == nil {
		t.Fatalf("bin expr type missing")
	}
	// Accept either usize or int per current M4 rules.
	if !(types.Equal(typ, types.USize) || types.Equal(typ, types.Int)) {
		t.Fatalf("u+2 type should be usize or int, got %s", typ.String())
	}
}
