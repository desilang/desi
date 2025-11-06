package check

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestDesugar_Map_ToListComp(t *testing.T) {
	// Build: main: map(xs, str)
	call := &ast.CallExpr{
		Callee: &ast.Ident{Name: "map"},
		Args:   []ast.Expr{&ast.Ident{Name: "xs"}, &ast.Ident{Name: "str"}},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: call}}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	// Desugar only (no type-checking)
	desugarMapFilter(mod)

	// Assert transformed shape: [ str(__x) for __x in xs ]
	es, ok := main.Body.Stmts[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("stmt[0] not ExprStmt, got %T", main.Body.Stmts[0])
	}
	lc, ok := es.Expr.(*ast.ListComp)
	if !ok {
		t.Fatalf("expr is not ListComp, got %T", es.Expr)
	}
	if len(lc.Clauses) != 1 {
		t.Fatalf("expected 1 clause, got %d", len(lc.Clauses))
	}
	cl := lc.Clauses[0]

	// Clause: Target == __x, Iter == xs, If == nil
	tid, ok := cl.Target.(*ast.Ident)
	if !ok || tid.Name != "__x" {
		t.Fatalf("clause.Target = %T (%v), want Ident __x", cl.Target, cl.Target)
	}
	iid, ok := cl.Iter.(*ast.Ident)
	if !ok || iid.Name != "xs" {
		t.Fatalf("clause.Iter = %T (%v), want Ident xs", cl.Iter, cl.Iter)
	}
	if cl.If != nil {
		t.Fatalf("clause.If = %T, want nil", cl.If)
	}

	// Elem: Call str(__x)
	ce, ok := lc.Elem.(*ast.CallExpr)
	if !ok {
		t.Fatalf("elem not CallExpr, got %T", lc.Elem)
	}
	cid, ok := ce.Callee.(*ast.Ident)
	if !ok || cid.Name != "str" {
		t.Fatalf("callee = %T (%v), want Ident 'str'", ce.Callee, ce.Callee)
	}
	if len(ce.Args) != 1 {
		t.Fatalf("want 1 arg, got %d", len(ce.Args))
	}
	arg, ok := ce.Args[0].(*ast.Ident)
	if !ok || arg.Name != "__x" {
		t.Fatalf("arg = %T (%v), want Ident __x", ce.Args[0], ce.Args[0])
	}
}

func TestDesugar_Filter_ToListComp(t *testing.T) {
	// Build: main: filter(xs, bool)
	call := &ast.CallExpr{
		Callee: &ast.Ident{Name: "filter"},
		Args:   []ast.Expr{&ast.Ident{Name: "xs"}, &ast.Ident{Name: "bool"}},
	}
	main := &ast.FuncDecl{
		Name: ast.Ident{Name: "main"},
		Body: &ast.Block{Stmts: []ast.Stmt{&ast.ExprStmt{Expr: call}}},
	}
	mod := &ast.Module{File: "<mem>", Decls: []ast.Decl{main}}

	// Desugar only (no type-checking)
	desugarMapFilter(mod)

	// Assert transformed shape: [ __x for __x in xs if bool(__x) ]
	es, ok := main.Body.Stmts[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("stmt[0] not ExprStmt, got %T", main.Body.Stmts[0])
	}
	lc, ok := es.Expr.(*ast.ListComp)
	if !ok {
		t.Fatalf("expr is not ListComp, got %T", es.Expr)
	}
	if len(lc.Clauses) != 1 {
		t.Fatalf("expected 1 clause, got %d", len(lc.Clauses))
	}
	cl := lc.Clauses[0]

	// Clause Target/Iter
	tid, ok := cl.Target.(*ast.Ident)
	if !ok || tid.Name != "__x" {
		t.Fatalf("clause.Target = %T (%v), want Ident __x", cl.Target, cl.Target)
	}
	iid, ok := cl.Iter.(*ast.Ident)
	if !ok || iid.Name != "xs" {
		t.Fatalf("clause.Iter = %T (%v), want Ident xs", cl.Iter, cl.Iter)
	}

	// If: Call(bool, __x)
	if cl.If == nil {
		t.Fatalf("clause.If is nil, want call to bool")
	}
	ic, ok := cl.If.(*ast.CallExpr)
	if !ok {
		t.Fatalf("clause.If not CallExpr, got %T", cl.If)
	}
	cid, ok := ic.Callee.(*ast.Ident)
	if !ok || cid.Name != "bool" {
		t.Fatalf("if callee = %T (%v), want Ident 'bool'", ic.Callee, ic.Callee)
	}
	if len(ic.Args) != 1 {
		t.Fatalf("if-call want 1 arg, got %d", len(ic.Args))
	}
	arg, ok := ic.Args[0].(*ast.Ident)
	if !ok || arg.Name != "__x" {
		t.Fatalf("if-call arg = %T (%v), want Ident __x", ic.Args[0], ic.Args[0])
	}

	// Elem should be the iteration variable itself
	eid, ok := lc.Elem.(*ast.Ident)
	if !ok || eid.Name != "__x" {
		t.Fatalf("elem = %T (%v), want Ident __x", lc.Elem, lc.Elem)
	}
}
