package parse

import "github.com/desilang/desi/compiler/internal/diag"

// DPE0005: invalid assignment target (matches codes.json)
func (p *Parser) errInvalidAssignTarget(sp diag.Span) {
	p.diags = append(p.diags, diag.Diagnostic{
		CodeID:  "DPE0005",
		Domain:  "parser",
		Title:   "invalid assignment target",
		Message: "only names, tuple patterns, or index/field targets may appear on the left of an assignment",
		Primary: diag.Label{Span: sp, Text: "parse error", Primary: true},
	})
}
