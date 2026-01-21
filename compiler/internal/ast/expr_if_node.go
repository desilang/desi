package ast

import "github.com/desilang/desi/compiler/internal/diag"

// IfExpr represents a ternary/conditional expression.
// Syntax: value_if_true if condition else value_if_false
// Example: 42 if is_valid else 0
type IfExpr struct {
	Then Expr      // value if condition is true
	Cond Expr      // the condition to evaluate
	Else Expr      // value if condition is false
	Span diag.Span // covers the entire expression
}

func (*IfExpr) isExpr()             {}
func (x *IfExpr) SpanOf() diag.Span { return x.Span }
