package ast

import "github.com/desilang/desi/compiler/internal/diag"

type CompClause struct {
	Target Expr
	Iter   Expr
	If     Expr // optional; nil if absent
}

type ListComp struct {
	Elem    Expr
	Clauses []CompClause
	Span    diag.Span
}

func (*ListComp) isExpr()             {}
func (e *ListComp) SpanOf() diag.Span { return e.Span }

type DictComp struct {
	Key     Expr
	Val     Expr
	Clauses []CompClause
	Span    diag.Span
}

func (*DictComp) isExpr()             {}
func (e *DictComp) SpanOf() diag.Span { return e.Span }

type SetComp struct {
	Elem    Expr
	Clauses []CompClause
	Span    diag.Span
}

func (*SetComp) isExpr()             {}
func (e *SetComp) SpanOf() diag.Span { return e.Span }
