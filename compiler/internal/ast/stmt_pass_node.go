package ast

import "github.com/desilang/desi/compiler/internal/diag"

// PassStmt is a no-op statement, equivalent to Python's 'pass'.
// Used as a placeholder in empty function/method bodies.
type PassStmt struct {
	Span diag.Span
}

// Implements Stmt interface
func (*PassStmt) isStmt() {}

// SpanOf returns the source span of this statement
func (p *PassStmt) SpanOf() diag.Span { return p.Span }
