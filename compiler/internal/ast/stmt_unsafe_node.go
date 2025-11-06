package ast

import "github.com/desilang/desi/compiler/internal/diag"

// UnsafeBlock represents:
//
//	unsafe:
//	  <indented block>
type UnsafeBlock struct {
	Body *Block
	Span diag.Span
}

func (*UnsafeBlock) isStmt()             {}
func (u *UnsafeBlock) SpanOf() diag.Span { return u.Span }
