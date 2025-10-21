package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parenLambdaAhead reports whether the current token is '(' and the token
// immediately following the *matching* ')' is a '=>'. It does not consume.
func (p *Parser) parenLambdaAhead() bool {
	if p.cur.Tok != token.LPAREN {
		return false
	}
	return p.afterMatchingParenIs(token.FAT_ARROW)
}

// parseLambdaFromParen parses a parenthesized lambda head followed by "=>":
//
//	(x:int, y:int) => expr
//	(x, y,) => expr
//
// Called only when parenLambdaAhead() returned true.
func (p *Parser) parseLambdaFromParen() ast.Expr {
	open := spanPos(p.file, p.cur)
	p.next() // consume '('

	var params []ast.LambdaParam
	if p.cur.Tok != token.RPAREN {
		for {
			if p.cur.Tok != token.IDENT {
				// Not a valid parameter head; consume until ')' and report a crisp diag.
				p.errLambdaParamsBeforeArrow(open)
				// recover to ')'
				for p.cur.Tok != token.RPAREN && p.cur.Tok != token.EOF {
					p.next()
				}
				break
			}
			start := spanPos(p.file, p.cur)
			name := ast.Ident{Name: p.cur.Lexeme, Span: start}
			p.next()

			var ty *ast.TypeName
			if p.accept(token.COLON) {
				ty = p.parseTypeName()
			}

			params = append(params, ast.LambdaParam{
				Name: name,
				Type: ty,
				Span: ast.JoinSpan(start, lastSpan(ty, start)),
			})

			if !p.accept(token.COMMA) {
				break
			}
			if p.cur.Tok == token.RPAREN {
				break // trailing comma ok
			}
		}
	}
	p.expectClose(token.RPAREN, ")", open)

	if !p.expect(token.FAT_ARROW, "=>") {
		// We already gated on afterMatchingParenIs, but keep a guard.
		p.errLambdaParamsBeforeArrow(open)
		return &ast.Ident{Name: "<error>", Span: open}
	}

	body := p.parseExpr()
	return &ast.LambdaExpr{
		Params: params,
		Body:   body,
		Span:   ast.JoinSpan(open, body.SpanOf()),
	}
}
