package parse

import "github.com/desilang/desi/compiler/internal/diag"

// DPE0002 is already used for generic “expected X”
// We'll add a focused helper for defer-not-call so tests can key on a stable message.
func (p *Parser) errDeferNeedsCall(sp diag.Span) {
	p.diags = append(p.diags, diag.Diagnostic{
		CodeID:  "DPE0002",
		Domain:  "parser",
		Title:   "expected call expression",
		Message: "defer requires a call expression like: defer fn(...)",
		Primary: diag.Label{Span: sp, Text: "not a call", Primary: true},
	})
}
