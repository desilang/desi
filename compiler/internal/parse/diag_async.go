package parse

import "github.com/desilang/desi/compiler/internal/diag"

// Async-specific diagnostics.
// NOTE: When we add 'async lambda' later, keep related diags here.

func (p *Parser) errAsyncBeforeDef(sp diag.Span) {
	p.diags = append(p.diags, diag.Diagnostic{
		CodeID:  "DPE1001",
		Domain:  "parser",
		Title:   "async only valid before 'def'",
		Message: "async only valid before 'def'",
		Primary: diag.Label{Span: sp, Text: "parse error", Primary: true},
	})
}

func (p *Parser) errAsyncBeforeLet(sp diag.Span) {
	p.diags = append(p.diags, diag.Diagnostic{
		CodeID:  "DPE1002",
		Domain:  "parser",
		Title:   "async not allowed before 'let'",
		Message: "async is not a statement modifier; remove it before 'let'",
		Primary: diag.Label{Span: sp, Text: "parse error", Primary: true},
	})
}
