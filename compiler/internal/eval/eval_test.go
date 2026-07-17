package eval

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/ast"
)

func TestEvalLiterals(t *testing.T) {
	env := NewEnv(nil)

	// Int Lit
	val, err := Eval(&ast.IntLit{Text: "42"}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := val.(IntValue); !ok || v.Val != 42 {
		t.Errorf("expected IntValue(42), got %T(%v)", val, val)
	}

	// Str Lit
	val, err = Eval(&ast.StrLit{Value: "hello"}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := val.(StrValue); !ok || v.Val != "hello" {
		t.Errorf("expected StrValue('hello'), got %T(%v)", val, val)
	}
}

func TestEvalLetAndIdent(t *testing.T) {
	env := NewEnv(nil)

	// let x = 42
	letStmt := &ast.LetStmt{
		Name:  ast.Ident{Name: "x"},
		Value: &ast.IntLit{Text: "42"},
	}

	_, err := Eval(letStmt, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// resolve x
	val, err := Eval(&ast.Ident{Name: "x"}, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := val.(IntValue); !ok || v.Val != 42 {
		t.Errorf("expected IntValue(42), got %T(%v)", val, val)
	}

	// undefined variable
	_, err = Eval(&ast.Ident{Name: "y"}, env)
	if err == nil {
		t.Fatal("expected error for undefined variable, got nil")
	}
}

func TestEvalBinaryExpr(t *testing.T) {
	env := NewEnv(nil)

	// 10 + 20
	binExpr := &ast.BinaryExpr{
		Op:  "+",
		Lhs: &ast.IntLit{Text: "10"},
		Rhs: &ast.IntLit{Text: "20"},
	}

	val, err := Eval(binExpr, env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := val.(IntValue); !ok || v.Val != 30 {
		t.Errorf("expected IntValue(30), got %T(%v)", val, val)
	}

	// 10 / 0
	divZero := &ast.BinaryExpr{
		Op:  "/",
		Lhs: &ast.IntLit{Text: "10"},
		Rhs: &ast.IntLit{Text: "0"},
	}

	_, err = Eval(divZero, env)
	if err == nil {
		t.Fatal("expected error for division by zero, got nil")
	}
}
