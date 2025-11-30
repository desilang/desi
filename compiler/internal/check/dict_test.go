package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

func TestDictLit_BasicTypeInference(t *testing.T) {
	// def main():
	//   let d = {"Alice": 100, "Bob": 85}
	dictLit := &ast.DictLit{
		Keys: []ast.Expr{
			&ast.StrLit{Value: "Alice"},
			&ast.StrLit{Value: "Bob"},
		},
		Values: []ast.Expr{
			&ast.IntLit{Text: "100"},
			&ast.IntLit{Text: "85"},
		},
	}

	letStmt := &ast.LetStmt{
		Name:  ast.Ident{Name: "d"},
		Type:  &ast.TypeName{Name: "dict", Params: []*ast.TypeName{{Name: "str"}, {Name: "int"}}},
		Value: dictLit,
	}

	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{letStmt}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}
	diags, info := Check(mod)

	mustNoDiags(t, diags)

	// Verify type is dict[str, int]
	dictType := info.Types[dictLit]
	if dictType == nil {
		t.Fatal("dict literal has no type")
	}

	dt, ok := dictType.(*types.Dict)
	if !ok {
		t.Fatalf("expected dict type, got %T", dictType)
	}

	if !types.Equal(dt.Key, types.Str) {
		t.Errorf("expected key type str, got %v", dt.Key)
	}

	if !types.Equal(dt.Val, types.Int) {
		t.Errorf("expected value type int, got %v", dt.Val)
	}
}

func TestDictLit_EmptyDict_Error(t *testing.T) {
	// def main():
	//   let d = {}  // Should error: needs type annotation
	dictLit := &ast.DictLit{
		Keys:   []ast.Expr{},
		Values: []ast.Expr{},
	}

	letStmt := &ast.LetStmt{
		Name:  ast.Ident{Name: "d"},
		Value: dictLit,
	}

	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{letStmt}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}
	diags, _ := Check(mod)

	if len(diags) == 0 {
		t.Fatal("expected error for empty dict literal")
	}
}

func TestDictLit_InconsistentKeyTypes_Error(t *testing.T) {
	// def main():
	//   let d = {"a": 1, 2: 3}  // Mixed str and int keys
	dictLit := &ast.DictLit{
		Keys: []ast.Expr{
			&ast.StrLit{Value: "a"},
			&ast.IntLit{Text: "2"},
		},
		Values: []ast.Expr{
			&ast.IntLit{Text: "1"},
			&ast.IntLit{Text: "3"},
		},
	}

	letStmt := &ast.LetStmt{
		Name:  ast.Ident{Name: "d"},
		Value: dictLit,
	}

	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{letStmt}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}
	diags, _ := Check(mod)

	mustHaveSomeDiagContaining(t, diags, "key type mismatch")
}

func TestDictLit_InconsistentValueTypes_Error(t *testing.T) {
	// def main():
	//   let d = {"a": 1, "b": "two"}  // Mixed int and str values
	dictLit := &ast.DictLit{
		Keys: []ast.Expr{
			&ast.StrLit{Value: "a"},
			&ast.StrLit{Value: "b"},
		},
		Values: []ast.Expr{
			&ast.IntLit{Text: "1"},
			&ast.StrLit{Value: "two"},
		},
	}

	letStmt := &ast.LetStmt{
		Name:  ast.Ident{Name: "d"},
		Value: dictLit,
	}

	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{letStmt}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}
	diags, _ := Check(mod)

	mustHaveSomeDiagContaining(t, diags, "value type mismatch")
}

func TestDictLit_SingleEntry(t *testing.T) {
	// def main():
	//   let d = {"key": 42}
	dictLit := &ast.DictLit{
		Keys: []ast.Expr{
			&ast.StrLit{Value: "key"},
		},
		Values: []ast.Expr{
			&ast.IntLit{Text: "42"},
		},
	}

	letStmt := &ast.LetStmt{
		Name:  ast.Ident{Name: "d"},
		Type:  &ast.TypeName{Name: "dict", Params: []*ast.TypeName{{Name: "str"}, {Name: "int"}}},
		Value: dictLit,
	}

	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{letStmt}},
	}

	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}
	diags, info := Check(mod)

	mustNoDiags(t, diags)

	// Verify type
	dictType := info.Types[dictLit]
	dt, ok := dictType.(*types.Dict)
	if !ok {
		t.Fatalf("expected dict type, got %T", dictType)
	}

	if !types.Equal(dt.Key, types.Str) || !types.Equal(dt.Val, types.Int) {
		t.Errorf("expected dict[str, int], got dict[%v, %v]", dt.Key, dt.Val)
	}
}
