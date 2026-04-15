package ast

import "github.com/desilang/desi/compiler/internal/diag"

// TryStmt represents try/except/finally error handling.
//
//	try:
//	    body
//	except [name]:
//	    handler
//	finally:
//	    cleanup
type TryStmt struct {
	Body       *Block    // try body (always present)
	ExceptVar  *Ident    // optional error binding in except clause (nil for bare except)
	ExceptType *Ident    // optional exception type in except clause (nil = catch all)
	Except     *Block    // except handler body (nil if no except clause)
	Finally    *Block    // finally body (nil if no finally clause)
	Span       diag.Span
}

func (*TryStmt) isStmt()             {}
func (s *TryStmt) SpanOf() diag.Span { return s.Span }

// RaiseStmt represents `raise expr`.
// Desugars to `return Err(expr)`.
type RaiseStmt struct {
	Value Expr      // the error expression to raise
	Span  diag.Span
}

func (*RaiseStmt) isStmt()             {}
func (s *RaiseStmt) SpanOf() diag.Span { return s.Span }
