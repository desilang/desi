package parse

import (
	"bytes"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseDecorators parses zero or more lines of decorators:
//
//	@name
//	@name(arg1, arg2)
//
// each terminated by NL. Returns the parsed decorators (possibly empty).
func (p *Parser) parseDecorators() []*ast.Decorator {
	var decs []*ast.Decorator
	for p.cur.Tok == token.AT {
		start := spanPos(p.file, p.cur) // '@'
		p.next()                        // consume '@'

		// dotted name: a.b.c
		dotName, nameSpan := p.parseDottedName()

		// optional arg list
		var args []ast.Expr
		if p.accept(token.LPAREN) {
			paren := spanPos(p.file, p.cur)
			if p.cur.Tok != token.RPAREN {
				for {
					args = append(args, p.parseExpr())
					if !p.accept(token.COMMA) {
						break
					}
					// allow trailing comma
					if p.cur.Tok == token.RPAREN {
						break
					}
				}
			}
			p.expectClose(token.RPAREN, ")", paren)
		}

		// require end-of-line after each decorator
		if !p.accept(token.NL) && p.cur.Tok != token.Indent && p.cur.Tok != token.EOF {
			p.errExpected(spanPos(p.file, p.cur), "newline")
		}

		decs = append(decs, &ast.Decorator{
			Name: ast.Ident{Name: dotName, Span: nameSpan},
			Args: args,
			Span: ast.JoinSpan(start, spanPos(p.file, p.cur)),
		})
	}
	return decs
}

// parseDottedName expects IDENT { '.' IDENT } and returns the full
// "a.b.c" spelling plus its span. On error, it emits a diagnostic and
// returns a placeholder name with a best-effort span.
func (p *Parser) parseDottedName() (string, diag.Span) {
	if p.cur.Tok != token.IDENT {
		sp := spanPos(p.file, p.cur)
		p.errExpected(sp, "identifier")
		return "<?>", sp
	}
	start := spanPos(p.file, p.cur)
	var b bytes.Buffer
	b.WriteString(p.cur.Lexeme)
	p.next()
	for p.accept(token.DOT) {
		if p.cur.Tok != token.IDENT {
			p.errExpected(spanPos(p.file, p.cur), "identifier after '.'")
			break
		}
		b.WriteByte('.')
		b.WriteString(p.cur.Lexeme)
		p.next()
	}
	return b.String(), ast.JoinSpan(start, spanPos(p.file, p.cur))
}
