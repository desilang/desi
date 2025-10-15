package parse

import (
	"bytes"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

func (p *Parser) parseFunc() *ast.FuncDecl {
	start := spanPos(p.file, p.cur)
	if !p.expect(token.KW_def, "def") {
		return nil
	}
	if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "function name")
		return nil
	}
	name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
	p.next()

	if !p.expect(token.LPAREN, "(") {
		return nil
	}
	var params []ast.Param
	if p.cur.Tok != token.RPAREN {
		params = p.parseParams()
	}
	p.expect(token.RPAREN, ")")

	var ret *ast.TypeName
	if p.accept(token.ARROW) {
		ret = p.parseTypeName()
	}

	// Body: ":" NL Block | NL
	var body *ast.Block
	if p.accept(token.COLON) {
		p.expect(token.NL, "newline")
		body = p.parseBlock()
	} else {
		p.expect(token.NL, "newline")
	}

	return &ast.FuncDecl{
		Name:    name,
		Params:  params,
		RetType: ret,
		Body:    body,
		Span:    ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}
}

func (p *Parser) parseParams() []ast.Param {
	var out []ast.Param
	for {
		if p.cur.Tok != token.IDENT {
			p.errUnexpected(spanPos(p.file, p.cur), "parameter")
			break
		}
		start := spanPos(p.file, p.cur)
		name := ast.Ident{Name: p.cur.Lexeme, Span: start}
		p.next()

		var ty *ast.TypeName
		if p.accept(token.COLON) {
			ty = p.parseTypeName()
		}

		var def ast.Expr
		if p.accept(token.ASSIGN) {
			def = p.parseExpr()
		}

		out = append(out, ast.Param{
			Name: name, Type: ty, Default: def,
			Span: ast.JoinSpan(start, lastSpan(def, name.Span)),
		})

		if !p.accept(token.COMMA) {
			break
		}
		if p.cur.Tok == token.RPAREN {
			break // trailing comma
		}
	}
	return out
}

func (p *Parser) parseTypeName() *ast.TypeName {
	if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "type name")
		return &ast.TypeName{Name: "<?>", Span: spanPos(p.file, p.cur)}
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
	return &ast.TypeName{Name: b.String(), Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
}
