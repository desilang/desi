package check

import (
	"math"
	"strconv"

	"github.com/desilang/desi/compiler/internal/ast"
)

// ConstKind represents the type of a constant value
type ConstKind int

const (
	InvalidConst ConstKind = iota
	IntConst
	FloatConst
	StrConst
	BoolConst
)

// ConstValue represents a compile-time constant value
type ConstValue struct {
	Kind  ConstKind
	Int   int64
	Float float64
	Str   string
	Bool  bool
}

// evalConst evaluates a constant expression.
// Returns nil if the expression is not a compile-time constant.
func (c *checker) evalConst(expr ast.Expr) *ConstValue {
	switch x := expr.(type) {
	case *ast.IntLit:
		val, err := strconv.ParseInt(x.Text, 0, 64)
		if err != nil {
			return nil
		}
		return &ConstValue{Kind: IntConst, Int: val}

	case *ast.FloatLit:
		val, err := strconv.ParseFloat(x.Text, 64)
		if err != nil {
			return nil
		}
		return &ConstValue{Kind: FloatConst, Float: val}

	case *ast.BoolLit:
		return &ConstValue{Kind: BoolConst, Bool: x.Value}

	case *ast.StrLit:
		// For string literals, use the source span text or Value field
		if x.Value != "" {
			return &ConstValue{Kind: StrConst, Str: x.Value}
		}
		// Fall back to extracting from span (basic case)
		return &ConstValue{Kind: StrConst, Str: ""}

	case *ast.UnaryExpr:
		return c.evalUnaryConst(x)

	case *ast.BinaryExpr:
		return c.evalBinaryConst(x)

	default:
		// Not a constant expression
		return nil
	}
}

// evalUnaryConst evaluates unary constant expressions
func (c *checker) evalUnaryConst(x *ast.UnaryExpr) *ConstValue {
	operand := c.evalConst(x.X)
	if operand == nil {
		return nil
	}

	switch x.Op {
	case "-":
		if operand.Kind == IntConst {
			return &ConstValue{Kind: IntConst, Int: -operand.Int}
		}
		if operand.Kind == FloatConst {
			return &ConstValue{Kind: FloatConst, Float: -operand.Float}
		}

	case "!", "not":
		if operand.Kind == BoolConst {
			return &ConstValue{Kind: BoolConst, Bool: !operand.Bool}
		}

	case "+":
		// Unary + is a no-op for numeric types
		if operand.Kind == IntConst || operand.Kind == FloatConst {
			return operand
		}
	}

	return nil
}

// evalBinaryConst evaluates binary constant expressions
func (c *checker) evalBinaryConst(x *ast.BinaryExpr) *ConstValue {
	lhs := c.evalConst(x.Lhs)
	rhs := c.evalConst(x.Rhs)
	if lhs == nil || rhs == nil {
		return nil
	}

	switch x.Op {
	// Arithmetic operations
	case "+":
		return c.evalAdd(lhs, rhs)
	case "-":
		return c.evalSub(lhs, rhs)
	case "*":
		return c.evalMul(lhs, rhs)
	case "/":
		return c.evalDiv(lhs, rhs)
	case "%":
		return c.evalMod(lhs, rhs)
	case "**":
		return c.evalPow(lhs, rhs)

	// Comparison operations
	case "==":
		return c.evalEq(lhs, rhs)
	case "!=":
		return c.evalNe(lhs, rhs)
	case "<":
		return c.evalLt(lhs, rhs)
	case "<=":
		return c.evalLe(lhs, rhs)
	case ">":
		return c.evalGt(lhs, rhs)
	case ">=":
		return c.evalGe(lhs, rhs)

	// Boolean operations
	case "and":
		if lhs.Kind == BoolConst && rhs.Kind == BoolConst {
			return &ConstValue{Kind: BoolConst, Bool: lhs.Bool && rhs.Bool}
		}
	case "or":
		if lhs.Kind == BoolConst && rhs.Kind == BoolConst {
			return &ConstValue{Kind: BoolConst, Bool: lhs.Bool || rhs.Bool}
		}
	}

	return nil
}

// evalAdd evaluates + for constants
func (c *checker) evalAdd(lhs, rhs *ConstValue) *ConstValue {
	if lhs.Kind == IntConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: IntConst, Int: lhs.Int + rhs.Int}
	}
	if lhs.Kind == FloatConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: FloatConst, Float: lhs.Float + rhs.Float}
	}
	// Mixed int/float promotes to float
	if lhs.Kind == IntConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: FloatConst, Float: float64(lhs.Int) + rhs.Float}
	}
	if lhs.Kind == FloatConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: FloatConst, Float: lhs.Float + float64(rhs.Int)}
	}
	// String concatenation
	if lhs.Kind == StrConst && rhs.Kind == StrConst {
		return &ConstValue{Kind: StrConst, Str: lhs.Str + rhs.Str}
	}
	return nil
}

// evalSub evaluates - for constants
func (c *checker) evalSub(lhs, rhs *ConstValue) *ConstValue {
	if lhs.Kind == IntConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: IntConst, Int: lhs.Int - rhs.Int}
	}
	if lhs.Kind == FloatConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: FloatConst, Float: lhs.Float - rhs.Float}
	}
	if lhs.Kind == IntConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: FloatConst, Float: float64(lhs.Int) - rhs.Float}
	}
	if lhs.Kind == FloatConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: FloatConst, Float: lhs.Float - float64(rhs.Int)}
	}
	return nil
}

// evalMul evaluates * for constants
func (c *checker) evalMul(lhs, rhs *ConstValue) *ConstValue {
	if lhs.Kind == IntConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: IntConst, Int: lhs.Int * rhs.Int}
	}
	if lhs.Kind == FloatConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: FloatConst, Float: lhs.Float * rhs.Float}
	}
	if lhs.Kind == IntConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: FloatConst, Float: float64(lhs.Int) * rhs.Float}
	}
	if lhs.Kind == FloatConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: FloatConst, Float: lhs.Float * float64(rhs.Int)}
	}
	return nil
}

// evalDiv evaluates / for constants
func (c *checker) evalDiv(lhs, rhs *ConstValue) *ConstValue {
	// Check for division by zero
	if rhs.Kind == IntConst && rhs.Int == 0 {
		return nil // Division by zero - not a valid constant
	}
	if rhs.Kind == FloatConst && rhs.Float == 0 {
		return nil
	}

	if lhs.Kind == IntConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: IntConst, Int: lhs.Int / rhs.Int}
	}
	if lhs.Kind == FloatConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: FloatConst, Float: lhs.Float / rhs.Float}
	}
	if lhs.Kind == IntConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: FloatConst, Float: float64(lhs.Int) / rhs.Float}
	}
	if lhs.Kind == FloatConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: FloatConst, Float: lhs.Float / float64(rhs.Int)}
	}
	return nil
}

// evalMod evaluates % for constants
func (c *checker) evalMod(lhs, rhs *ConstValue) *ConstValue {
	if lhs.Kind == IntConst && rhs.Kind == IntConst {
		if rhs.Int == 0 {
			return nil // Modulo by zero
		}
		return &ConstValue{Kind: IntConst, Int: lhs.Int % rhs.Int}
	}
	return nil
}

// evalPow evaluates ** for constants
func (c *checker) evalPow(lhs, rhs *ConstValue) *ConstValue {
	if lhs.Kind == IntConst && rhs.Kind == IntConst {
		// For integer exponentiation with non-negative exponent
		if rhs.Int >= 0 {
			result := int64(1)
			base := lhs.Int
			exp := rhs.Int
			for exp > 0 {
				if exp%2 == 1 {
					result *= base
				}
				base *= base
				exp /= 2
			}
			return &ConstValue{Kind: IntConst, Int: result}
		}
		// Negative exponent for integers -> use float
		return &ConstValue{Kind: FloatConst, Float: math.Pow(float64(lhs.Int), float64(rhs.Int))}
	}
	if lhs.Kind == FloatConst || rhs.Kind == FloatConst {
		l := lhs.Float
		r := rhs.Float
		if lhs.Kind == IntConst {
			l = float64(lhs.Int)
		}
		if rhs.Kind == IntConst {
			r = float64(rhs.Int)
		}
		return &ConstValue{Kind: FloatConst, Float: math.Pow(l, r)}
	}
	return nil
}

// evalEq evaluates == for constants
func (c *checker) evalEq(lhs, rhs *ConstValue) *ConstValue {
	if lhs.Kind == IntConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Int == rhs.Int}
	}
	if lhs.Kind == FloatConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Float == rhs.Float}
	}
	if lhs.Kind == BoolConst && rhs.Kind == BoolConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Bool == rhs.Bool}
	}
	if lhs.Kind == StrConst && rhs.Kind == StrConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Str == rhs.Str}
	}
	return nil
}

// evalNe evaluates != for constants
func (c *checker) evalNe(lhs, rhs *ConstValue) *ConstValue {
	eq := c.evalEq(lhs, rhs)
	if eq != nil {
		return &ConstValue{Kind: BoolConst, Bool: !eq.Bool}
	}
	return nil
}

// evalLt evaluates < for constants
func (c *checker) evalLt(lhs, rhs *ConstValue) *ConstValue {
	if lhs.Kind == IntConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Int < rhs.Int}
	}
	if lhs.Kind == FloatConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Float < rhs.Float}
	}
	if lhs.Kind == StrConst && rhs.Kind == StrConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Str < rhs.Str}
	}
	return nil
}

// evalLe evaluates <= for constants
func (c *checker) evalLe(lhs, rhs *ConstValue) *ConstValue {
	if lhs.Kind == IntConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Int <= rhs.Int}
	}
	if lhs.Kind == FloatConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Float <= rhs.Float}
	}
	if lhs.Kind == StrConst && rhs.Kind == StrConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Str <= rhs.Str}
	}
	return nil
}

// evalGt evaluates > for constants
func (c *checker) evalGt(lhs, rhs *ConstValue) *ConstValue {
	if lhs.Kind == IntConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Int > rhs.Int}
	}
	if lhs.Kind == FloatConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Float > rhs.Float}
	}
	if lhs.Kind == StrConst && rhs.Kind == StrConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Str > rhs.Str}
	}
	return nil
}

// evalGe evaluates >= for constants
func (c *checker) evalGe(lhs, rhs *ConstValue) *ConstValue {
	if lhs.Kind == IntConst && rhs.Kind == IntConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Int >= rhs.Int}
	}
	if lhs.Kind == FloatConst && rhs.Kind == FloatConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Float >= rhs.Float}
	}
	if lhs.Kind == StrConst && rhs.Kind == StrConst {
		return &ConstValue{Kind: BoolConst, Bool: lhs.Str >= rhs.Str}
	}
	return nil
}
