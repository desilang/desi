package ast

import "github.com/desilang/desi/compiler/internal/diag"

// AssignStmt handles ':=' with comma-separated lists on both sides.
type AssignStmt struct {
	LHS  []Expr
	RHS  []Expr
	Span diag.Span
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
