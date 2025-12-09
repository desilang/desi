package ast

import "github.com/desilang/desi/compiler/internal/diag"

type IfArm struct {
	Cond Expr
	Body *Block
}

type IfStmt struct {
	Cond  Expr
	Then  *Block
	Elifs []IfArm
	Else  *Block // optional
	Span  diag.Span
}

func (*IfStmt) isStmt()             {}
func (s *IfStmt) SpanOf() diag.Span { return s.Span }

type WhileStmt struct {
	Cond Expr
	Body *Block
	Span diag.Span
}

func (*WhileStmt) isStmt()             {}
func (s *WhileStmt) SpanOf() diag.Span { return s.Span }

// ForTarget represents a single loop variable binding (name and optional type)
type ForTarget struct {
	Name *Ident    // Variable name
	Type *TypeName // Optional type annotation (nil if not specified)
}

type ForStmt struct {
	Target  Expr        // Ident | tuple-like | list-like; parsed-only in M2 (legacy, kept for backward compat)
	Targets []ForTarget // New: typed bindings for `for x: T in ...` or `for k: K, v: V in ...`
	Iter    Expr
	Body    *Block
	Span    diag.Span
}

func (*ForStmt) isStmt()             {}
func (s *ForStmt) SpanOf() diag.Span { return s.Span }
