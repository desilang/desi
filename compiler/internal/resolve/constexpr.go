package resolve

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/types"
)

// ConstArithType reports the type of an expression built only from literals and
// the arithmetic operators, or nil when the expression is not one. Mixing an int
// and a float promotes to float, matching arithmetic elsewhere, and '+' over two
// string literals is a string.
//
// Module-level constants need this on both sides of a module boundary: the
// exporter records the type here, the importer binds the name with it, and the
// lower package folds the matching value. All three must agree on which
// initializers count as constant.
func ConstArithType(e ast.Expr) types.T {
	switch v := e.(type) {
	case *ast.IntLit:
		return types.Int
	case *ast.FloatLit:
		return types.Float
	case *ast.StrLit:
		return types.Str
	case *ast.UnaryExpr:
		if v.Op != "-" {
			return nil
		}
		// Negation is numeric only — "-" on a string is not a constant.
		t := ConstArithType(v.X)
		if types.Equal(t, types.Int) || types.Equal(t, types.Float) {
			return t
		}
		return nil
	case *ast.BinaryExpr:
		switch v.Op {
		case "+", "-", "*", "/", "%":
		default:
			return nil
		}
		lt := ConstArithType(v.Lhs)
		rt := ConstArithType(v.Rhs)
		if lt == nil || rt == nil {
			return nil
		}
		// Concatenation: only '+', and only when both sides are strings.
		if types.Equal(lt, types.Str) || types.Equal(rt, types.Str) {
			if v.Op == "+" && types.Equal(lt, types.Str) && types.Equal(rt, types.Str) {
				return types.Str
			}
			return nil
		}
		if types.Equal(lt, types.Float) || types.Equal(rt, types.Float) {
			return types.Float
		}
		return types.Int
	}
	return nil
}
