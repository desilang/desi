package ast

import "github.com/desilang/desi/compiler/internal/diag"

// TryExpr represents the postfix `?` operator for error propagation.
// expr? desugars to: match expr { Ok(v) => v, Err(e) => return Err(e) }
// For Option: Some(v) => v, Nothing => return Nothing
type TryExpr struct {
	X    Expr      // the expression being tried
	Span diag.Span // covers expr?
}

func (*TryExpr) isExpr()             {}
func (x *TryExpr) SpanOf() diag.Span { return x.Span }
