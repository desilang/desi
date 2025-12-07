package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseTypeAlias parses: ["pub"] "type" Ident [ "<" TypeParams ">" ] "=" TypeExpr NL
func (p *Parser) parseTypeAlias(pub bool) *ast.TypeAliasDecl {
	start := spanPos(p.file, p.cur)

	// If pub was passed, consume "pub" first
	if pub {
		if !p.expect(token.KW_pub, "pub") {
			return nil
		}
	}

	if !p.expect(token.KW_type, "type") {
		return nil
	}

	// Parse name
	if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "type alias name")
		return nil
	}
	name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
	p.next()

	// Parse optional type parameters: <T> or <T, U>
	var typeParams []ast.Ident
	if p.cur.Tok == token.LT { // <
		p.next()
		for {
			if p.cur.Tok != token.IDENT {
				p.errExpected(spanPos(p.file, p.cur), "type parameter name")
				break
			}
			typeParams = append(typeParams, ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)})
			p.next()

			if p.cur.Tok == token.GT { // >
				p.next()
				break
			}
			if !p.expect(token.COMMA, ",") {
				break
			}
		}
	}

	// Expect "="
	if !p.expect(token.ASSIGN, "=") {
		p.syncStmt()
		return nil
	}

	// Parse target type expression
	target := p.parseTypeName()
	if target == nil {
		p.errExpected(spanPos(p.file, p.cur), "type expression")
		return nil
	}

	// Expect newline
	if !p.accept(token.NL) && p.cur.Tok != token.EOF {
		p.errExpected(spanPos(p.file, p.cur), "newline")
	}

	return &ast.TypeAliasDecl{
		Pub:        pub,
		Name:       name,
		TypeParams: typeParams,
		Target:     target,
		Span:       ast.JoinSpan(start, target.Span),
	}
}
