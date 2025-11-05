package ast

import "github.com/desilang/desi/compiler/internal/diag"

// UnsafeBlock wraps a nested block where unsafe operations are permitted.
// It has block semantics (no implicit value).
type UnsafeBlock struct {
	Body *Block
	Span diag.Span
}

func (*UnsafeBlock) isStmt()             {}
func (s *UnsafeBlock) SpanOf() diag.Span { return s.Span }
