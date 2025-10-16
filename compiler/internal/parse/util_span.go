package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// lastSpan returns n.SpanOf() or an empty span if n is nil.
func lastSpan(n ast.Node, fallback diag.Span) diag.Span {
	if n == nil {
		return fallback
	}
	return n.SpanOf()
}
