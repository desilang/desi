package ast

import "github.com/desilang/desi/compiler/internal/diag"

type MatchArm struct {
	Pattern Expr
	Result  Expr
	Span    diag.Span
}

type MatchStmt struct {
	Scrutinee Expr
	Arms      []MatchArm
	Span      diag.Span
}

func (m *MatchStmt) SpanOf() diag.Span { return m.Span }
