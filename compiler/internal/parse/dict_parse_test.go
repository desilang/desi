package parse

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestParse_DictLiteral_MultiStmt(t *testing.T) {
	src := `
def main():
    # Basic dict literal
    let scores = {"Alice": 100, "Bob": 85}
    
    # Single entry dict
    let single = {"key": 42}
`
	mod, diags := ParseFile("test.desi", []byte(src))
	if len(diags) > 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}

	if len(mod.Decls) != 1 {
		t.Fatalf("want 1 decl, got %d", len(mod.Decls))
	}
	fn := mod.Decls[0].(*ast.FuncDecl)
	if len(fn.Body.Stmts) != 2 {
		t.Fatalf("want 2 stmts, got %d", len(fn.Body.Stmts))
	}

	s1, ok := fn.Body.Stmts[0].(*ast.LetStmt)
	if !ok {
		t.Fatalf("stmt 1 not let, got %T", fn.Body.Stmts[0])
	}
	if s1.Name.Name != "scores" {
		t.Errorf("stmt 1 name = %s, want scores", s1.Name.Name)
	}

	s2, ok := fn.Body.Stmts[1].(*ast.LetStmt)
	if !ok {
		t.Fatalf("stmt 2 not let, got %T", fn.Body.Stmts[1])
	}
	if s2.Name.Name != "single" {
		t.Errorf("stmt 2 name = %s, want single", s2.Name.Name)
	}
}

func TestParse_DictLiteral_MultiLine(t *testing.T) {
	src := `
def main():
    let d = {
        "a": 1,
        "b": 2
    }
    let x = 1
`
	mod, diags := ParseFile("test.desi", []byte(src))
	if len(diags) > 0 {
		t.Fatalf("unexpected diags: %v", diags)
	}

	fn := mod.Decls[0].(*ast.FuncDecl)
	if len(fn.Body.Stmts) != 2 {
		t.Fatalf("want 2 stmts, got %d", len(fn.Body.Stmts))
	}
}
