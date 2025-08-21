package parser

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestParseExprsInFunction(t *testing.T) {
	src := "" +
		"def f(a: i32) -> i32:\n" +
		"  let mut x = 1 + 2 * 3\n" +
		"  x := (x + 1) * 2\n" +
		"  return x\n"

	p := New(src)
	f, err := p.ParseFile()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(f.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(f.Decls))
	}
	fn, ok := f.Decls[0].(*ast.FuncDecl)
	if !ok {
		t.Fatalf("decl 0 not a FuncDecl")
	}
	if len(fn.Body) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(fn.Body))
	}

	// let mut x = 1 + 2 * 3
	let1, ok := fn.Body[0].(*ast.LetStmt)
	if !ok || !let1.Mutable {
		t.Fatalf("stmt0 not LetStmt(mut)")
	}
	plus, ok := let1.Expr.(*ast.BinaryExpr)
	if !ok || plus.Op != "+" {
		t.Fatalf("let1 expr not Binary '+'")
	}
	times, ok := plus.Right.(*ast.BinaryExpr)
	if !ok || times.Op != "*" {
		t.Fatalf("right child not '*'")
	}

	// x := (x + 1) * 2
	asg, ok := fn.Body[1].(*ast.AssignStmt)
	if !ok {
		t.Fatalf("stmt1 not AssignStmt")
	}
	mul2, ok := asg.Expr.(*ast.BinaryExpr)
	if !ok || mul2.Op != "*" {
		t.Fatalf("assign expr not Binary '*'")
	}
}
