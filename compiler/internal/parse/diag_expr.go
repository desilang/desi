package parse

import "github.com/desilang/desi/compiler/internal/diag"

// Focused diagnostics for expr-specific situations.

func (p *Parser) errLambdaParamsBeforeArrow(sp diag.Span) {
	p.diags = append(p.diags, diag.Diagnostic{
		CodeID:  "DPE0002",
		Domain:  "parser",
		Title:   "lambda parameter list expected",
		Message: "lambda parameter list expected before '=>'",
		Primary: diag.Label{Span: sp, Text: "parse error", Primary: true},
	})
}
