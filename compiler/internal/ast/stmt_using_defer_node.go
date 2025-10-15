package ast

import "github.com/desilang/desi/compiler/internal/diag"

// UsingStmt supports either binding form "using x = Expr: NL Block"
// or a naked target (left for future forms). Init may be nil.
type UsingStmt struct {
	Bind Expr // Ident | FieldExpr | IndexExpr
	Init Expr // nil if no "= Expr" was present
	Body *Block
	Span diag.Span
}

func (*UsingStmt) isStmt()             {}
func (s *UsingStmt) SpanOf() diag.Span { return s.Span }

// DeferStmt requires a call in parse; checker may enforce more later.
type DeferStmt struct {
	Call *CallExpr
	Span diag.Span
}

func (*DeferStmt) isStmt()             {}
func (s *DeferStmt) SpanOf() diag.Span { return s.Span }
