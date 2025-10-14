package parse

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/diag"
)

func (p *Parser) errExpected(sp diag.Span, want string) {
	p.diags = append(p.diags, diag.Diagnostic{
		CodeID:  "DPE0002",
		Domain:  "parser",
		Title:   "expected a different token",
		Message: fmt.Sprintf("expected %s", want),
		Primary: diag.Label{Span: sp, Text: "parse error", Primary: true},
	})
}

func (p *Parser) errUnexpected(sp diag.Span, ctx string) {
	msg := "unexpected token"
	if ctx != "" {
		msg = "unexpected token while parsing " + ctx
	}
	p.diags = append(p.diags, diag.Diagnostic{
		CodeID:  "DPE0001",
		Domain:  "parser",
		Title:   "unexpected token",
		Message: msg,
		Primary: diag.Label{Span: sp, Text: "parse error", Primary: true},
	})
}
