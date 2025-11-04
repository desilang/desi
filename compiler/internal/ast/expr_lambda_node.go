package ast

import "github.com/desilang/desi/compiler/internal/diag"

// Minimal lambda params for M2.
type LambdaParam struct {
	Name Ident
	Type *TypeName // optional
	Span diag.Span
}

type LambdaExpr struct {
	Async  bool
	Params []LambdaParam
	Body   Expr
	Span   diag.Span
}

func (*LambdaExpr) isExpr()             {}
func (e *LambdaExpr) SpanOf() diag.Span { return e.Span }
