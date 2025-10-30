package ast

import "github.com/desilang/desi/compiler/internal/diag"

// SliceExpr models s[i:j:k]; any of I/J/K may be nil (omitted).
type SliceExpr struct {
	X    Expr
	I    Expr // start (optional)
	J    Expr // stop  (optional)
	K    Expr // step  (optional)
	Span diag.Span
}

func (*SliceExpr) isExpr()             {}
func (e *SliceExpr) SpanOf() diag.Span { return e.Span }
