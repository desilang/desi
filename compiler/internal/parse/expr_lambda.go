package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parse lambda with IDENT head:  Ident "=>" Expr
// (Typed or multi-parameter lambdas are supported via the parenthesized form
// in a future enhancement; this path handles the untyped single-param case.)
func (p *Parser) parseLambdaFromIdent() ast.Expr {
	// ident
	id := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
	p.next()

	if !p.expect(token.FAT_ARROW, "=>") {
		// Not actually a lambda; treat as a plain Ident expr.
		return &id
	}
	body := p.parseExpr()

	lp := ast.LambdaParam{
		Name: id,
		Span: id.Span,
	}
	return &ast.LambdaExpr{
		Params: []ast.LambdaParam{lp},
		Body:   body,
		Span:   ast.JoinSpan(id.Span, body.SpanOf()),
	}
}
