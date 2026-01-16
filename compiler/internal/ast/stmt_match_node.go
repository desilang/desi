package ast

import "github.com/desilang/desi/compiler/internal/diag"

type MatchArm struct {
	Pattern Expr
	Guard   Expr // Optional guard condition (nil if no guard)
	Result  Expr
	Span    diag.Span
}

type MatchExpr struct {
	Scrutinee Expr
	Arms      []MatchArm
	Span      diag.Span
}

func (m *MatchExpr) SpanOf() diag.Span { return m.Span }

// --- satisfy the Expr interface ---
func (*MatchExpr) isExpr() {}
func (*MatchExpr) isStmt() {} // Also satisfy Stmt for backward compatibility if needed, but better to wrap in ExprStmt
