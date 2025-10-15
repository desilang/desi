package ast

import "github.com/desilang/desi/compiler/internal/diag"

type UsingStmt struct {
	Target Expr // either Ident "=" CallExpr OR a general LHS
	Body   *Block
	Span   diag.Span
}

func (*UsingStmt) isStmt()             {}
func (s *UsingStmt) SpanOf() diag.Span { return s.Span }

type DeferStmt struct {
	Call *CallExpr
	Span diag.Span
}

func (*DeferStmt) isStmt()             {}
func (s *DeferStmt) SpanOf() diag.Span { return s.Span }
