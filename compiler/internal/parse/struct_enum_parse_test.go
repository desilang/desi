package parse

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestM3B_StructEnum_Parse(t *testing.T) {
	src := `
@serde
pub struct User:
	"""user doc"""
	pub id: u64
	name: str

@flags
enum Mode:
	"""mode doc"""
	Auto: none
	Manual: str
`
	mod, diags := ParseFile("<mem>", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
	if len(mod.Decls) != 2 {
		t.Fatalf("expected 2 decls, got %d", len(mod.Decls))
	}

	// Struct
	st, ok := mod.Decls[0].(*ast.StructDecl)
	if !ok {
		t.Fatalf("decl 0 not a StructDecl: %T", mod.Decls[0])
	}
	if !st.Pub || st.Name.Name != "User" {
		t.Fatalf("struct visibility/name wrong: pub=%v name=%s", st.Pub, st.Name.Name)
	}
	if st.Doc == nil || !st.Doc.Long {
		t.Fatalf("struct docstring not attached")
	}
	if len(st.Fields) != 2 {
		t.Fatalf("struct fields wrong: got %d", len(st.Fields))
	}
	if !st.Fields[0].Pub || st.Fields[0].Name.Name != "id" || st.Fields[0].Type == nil || st.Fields[0].Type.Name != "u64" {
		t.Fatalf("field 0 invalid: %+v", st.Fields[0])
	}
	if st.Fields[1].Pub || st.Fields[1].Name.Name != "name" || st.Fields[1].Type == nil || st.Fields[1].Type.Name != "str" {
		t.Fatalf("field 1 invalid: %+v", st.Fields[1])
	}

	// Enum
	en, ok := mod.Decls[1].(*ast.EnumDecl)
	if !ok {
		t.Fatalf("decl 1 not an EnumDecl: %T", mod.Decls[1])
	}
	if en.Pub {
		t.Fatalf("enum should be private by default (no 'pub')")
	}
	if en.Doc == nil || !en.Doc.Long {
		t.Fatalf("enum docstring not attached")
	}
	if len(en.Variants) != 2 {
		t.Fatalf("enum variants wrong: got %d", len(en.Variants))
	}
	if en.Variants[0].Name.Name != "Auto" || en.Variants[0].Type != nil {
		t.Fatalf("variant 0 invalid: %+v", en.Variants[0])
	}
	if en.Variants[1].Name.Name != "Manual" || en.Variants[1].Type == nil || en.Variants[1].Type.Name != "str" {
		t.Fatalf("variant 1 invalid: %+v", en.Variants[1])
	}
}
