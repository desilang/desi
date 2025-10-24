package parse

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// import dotted.name [as alias]
func (p *Parser) parseImport() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'import'
	p.next()                        // consume 'import'

	// dotted.name
	dotted, nameSpan := p.parseDottedName()
	segments := strings.Split(dotted, ".")

	// optional "as alias"
	var alias *ast.Ident
	if p.accept(token.KW_as) {
		if p.cur.Tok != token.IDENT {
			p.errExpected(spanPos(p.file, p.cur), "identifier")
		} else {
			a := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
			alias = &a
			p.next()
		}
	}

	// End of statement
	if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
		p.errExpected(spanPos(p.file, p.cur), "newline")
	}

	return &ast.ImportStmt{
		Path:  segments,
		Alias: alias,
		Span:  ast.JoinSpan(start, lastSpan(alias, nameSpan)),
	}
}

// from dotted.name import a [as x], b, ...
func (p *Parser) parseFromImport() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'from'
	p.next()                        // consume 'from'

	// dotted.name
	dotted, nameSpan := p.parseDottedName()
	segments := strings.Split(dotted, ".")

	if !p.expect(token.KW_import, "import") {
		p.syncStmt()
		return nil
	}

	var items []ast.FromImportItem
	for {
		if p.cur.Tok != token.IDENT {
			p.errExpected(spanPos(p.file, p.cur), "identifier")
			break
		}
		n := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
		p.next()

		var alias *ast.Ident
		if p.accept(token.KW_as) {
			if p.cur.Tok != token.IDENT {
				p.errExpected(spanPos(p.file, p.cur), "identifier")
			} else {
				a := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
				alias = &a
				p.next()
			}
		}

		itemEnd := n.Span
		if alias != nil {
			itemEnd = alias.Span
		}
		items = append(items, ast.FromImportItem{
			Name:  n,
			Alias: alias,
			Span:  ast.JoinSpan(n.Span, itemEnd),
		})

		if !p.accept(token.COMMA) {
			break
		}
		// allow trailing comma before newline/dedent/eof
		if p.cur.Tok == token.NL || p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
			break
		}
	}

	// End of statement
	if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
		p.errExpected(spanPos(p.file, p.cur), "newline")
	}

	end := nameSpan
	if len(items) > 0 {
		last := items[len(items)-1]
		end = last.Span
	}
	return &ast.FromImportStmt{
		Path:  segments,
		Items: items,
		Span:  ast.JoinSpan(start, end),
	}
}
