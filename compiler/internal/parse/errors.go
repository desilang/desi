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

// errUnclosed reports an unclosed delimiter with the primary span pointing at the opener.
func (p *Parser) errUnclosed(openSpan diag.Span, delim string) {
	msg := "unclosed delimiter"
	if delim != "" {
		msg = "unclosed " + delim
	}
	p.diags = append(p.diags, diag.Diagnostic{
		CodeID:  "DPE0003",
		Domain:  "parser",
		Title:   "unclosed delimiter",
		Message: msg,
		Primary: diag.Label{Span: openSpan, Text: "parse error", Primary: true},
	})
}
