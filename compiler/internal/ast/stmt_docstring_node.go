package ast

import "github.com/desilang/desi/compiler/internal/diag"

// DocStringStmt is a bare string as the first statement in a block.
type DocStringStmt struct {
	Value *StrLit
	Span  diag.Span
}

func (*DocStringStmt) isStmt()             {}
func (s *DocStringStmt) SpanOf() diag.Span { return s.Span }
