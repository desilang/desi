package parse

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// import dotted.name [as alias]
// import .relative_module [as alias]  (relative import)
func (p *Parser) parseImport() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'import'
	p.next()                        // consume 'import'

	// Check for relative import: import .module
	relative := false
	if p.cur.Tok == token.DOT {
		relative = true
		p.next() // consume leading '.'
	}

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
		Path:     segments,
		Alias:    alias,
		Relative: relative,
		Span:     ast.JoinSpan(start, lastSpan(alias, nameSpan)),
	}
}

// from dotted.name import a [as x], b, ...
// from .relative_module import a [as x], ...  (relative import)
func (p *Parser) parseFromImport() ast.Stmt {
	start := spanPos(p.file, p.cur) // 'from'
	p.next()                        // consume 'from'

	// Check for relative import: from .module import ...
	relative := false
	if p.cur.Tok == token.DOT {
		relative = true
		p.next() // consume leading '.'
	}

	// dotted.name
	dotted, nameSpan := p.parseDottedName()
	segments := strings.Split(dotted, ".")

	if !p.expect(token.KW_import, "import") {
		p.syncStmt()
		return nil
	}

	// Check for wildcard import: from X import *
	if p.cur.Tok == token.STAR {
		starSpan := spanPos(p.file, p.cur)
		p.next() // consume '*'

		// End of statement
		if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
			p.errExpected(spanPos(p.file, p.cur), "newline")
		}

		return &ast.FromImportStmt{
			Path:     segments,
			Star:     true,
			Relative: relative,
			Span:     ast.JoinSpan(start, starSpan),
		}
	}

	var items []ast.FromImportItem
	for {
		if p.cur.Tok != token.IDENT {
			p.errExpected(spanPos(p.file, p.cur), "identifier")
			break
		}

		// Parse dotted name (e.g., Container.Item)
		var path []string
		startSpan := spanPos(p.file, p.cur)
		path = append(path, p.cur.Lexeme)
		p.next()

		// Continue parsing if there's a DOT
		for p.cur.Tok == token.DOT {
			p.next() // consume '.'
			if p.cur.Tok != token.IDENT {
				p.errExpected(spanPos(p.file, p.cur), "identifier")
				break
			}
			path = append(path, p.cur.Lexeme)
			p.next()
		}

		// Name is the last segment (for display/alias purposes)
		lastName := path[len(path)-1]
		n := ast.Ident{Name: lastName, Span: startSpan}

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
			Path:  path,
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
		Path:     segments,
		Items:    items,
		Relative: relative,
		Span:     ast.JoinSpan(start, end),
	}
}
