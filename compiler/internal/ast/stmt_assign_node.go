package ast

import "github.com/desilang/desi/compiler/internal/diag"

// AssignStmt handles both '=' (initial bind) and ':=' (reassignment).
// IsReassign is true when ':=' was used (mutation), false for '=' (binding).
type AssignStmt struct {
	LHS        []Expr
	RHS        []Expr
	IsReassign bool // true for ':=', false for '='
	Span       diag.Span
}

func (*AssignStmt) isStmt()             {}
func (s *AssignStmt) SpanOf() diag.Span { return s.Span }

// AugAssignStmt handles 'x += y', 'x **= y', etc.
type AugAssignStmt struct {
	Op    string // "+=", "-=", "*=", "/=", "%=", "**=", "^="
	Left  Expr
	Right Expr
	Span  diag.Span
}

func (*AugAssignStmt) isStmt()             {}
func (s *AugAssignStmt) SpanOf() diag.Span { return s.Span }
