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
	if len(let1.Binds) != 1 || let1.Binds[0].Name != "x" {
		t.Fatalf("let1 binds = %#v", let1.Binds)
	}
	if len(let1.Values) != 1 {
		t.Fatalf("let1 values len != 1")
	}
	plus, ok := let1.Values[0].(*ast.BinaryExpr)
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
	if len(asg.Names) != 1 || asg.Names[0] != "x" {
		t.Fatalf("assign LHS = %#v", asg.Names)
	}
	if len(asg.Exprs) != 1 {
		t.Fatalf("assign RHS len != 1")
	}
	mul2, ok := asg.Exprs[0].(*ast.BinaryExpr)
	if !ok || mul2.Op != "*" {
		t.Fatalf("assign expr not Binary '*'")
	}
}

func TestParallelLetAndAssign(t *testing.T) {
	src := "" +
		"def g() -> void:\n" +
		"  let a, b:int, c = 1, 2, 3\n" +
		"  a, b := b, a\n" +
		"  return\n"

	p := New(src)
	f, err := p.ParseFile()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	fn := f.Decls[0].(*ast.FuncDecl)
	if len(fn.Body) != 3 {
		t.Fatalf("expected 3 stmts, got %d", len(fn.Body))
	}
	// let a, b:int, c = 1, 2, 3
	let0 := fn.Body[0].(*ast.LetStmt)
	if len(let0.Binds) != 3 {
		t.Fatalf("bind count = %d", len(let0.Binds))
	}
	if let0.Binds[1].Type != "int" {
		t.Fatalf("per-name type not captured: %#v", let0.Binds[1])
	}
	if len(let0.Values) != 3 {
		t.Fatalf("RHS count = %d", len(let0.Values))
	}
	// a, b := b, a
	asg := fn.Body[1].(*ast.AssignStmt)
	if len(asg.Names) != 2 || asg.Names[0] != "a" || asg.Names[1] != "b" {
		t.Fatalf("assign names: %#v", asg.Names)
	}
	if len(asg.Exprs) != 2 {
		t.Fatalf("assign exprs: %#v", asg.Exprs)
	}
}
