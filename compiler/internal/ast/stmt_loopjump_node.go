package ast

import "github.com/desilang/desi/compiler/internal/diag"

// BreakStmt leaves the innermost enclosing loop.
type BreakStmt struct {
	Span diag.Span
}

func (*BreakStmt) isStmt() {}

// SpanOf returns the source span of this statement
func (s *BreakStmt) SpanOf() diag.Span { return s.Span }

// ContinueStmt skips to the next iteration of the innermost enclosing loop.
type ContinueStmt struct {
	Span diag.Span
}

func (*ContinueStmt) isStmt() {}

// SpanOf returns the source span of this statement
func (s *ContinueStmt) SpanOf() diag.Span { return s.Span }
