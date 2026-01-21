package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// Parse list/dict/set comprehensions in expression position.
//
// List: [ elem for target in iter { for target in iter } [ if expr ] ]
// Dict: { key: val for target in iter { ... } [ if expr ] }
// Set : #{ elem for target in iter { ... } [ if expr ] }

func (p *Parser) parseListLiteralOrComp(openSpan ast.Node) ast.Expr {
	// Called with current token after '['.

	// Skip any leading newlines for multiline support
	p.skipNLs()

	// Handle empty list: []
	if p.cur.Tok == token.RBRACK {
		end := spanPos(p.file, p.cur)
		p.next()
		return &ast.ListLit{
			Elems: nil,
			Span:  ast.JoinSpan(openSpan.SpanOf(), end),
		}
	}

	elem := p.parseExpr()

	// Check if it's a comprehension (has 'for')
	if p.cur.Tok == token.KW_for {
		p.next() // consume 'for'
		clauses := p.parseCompClausesAfterFor()
		if !p.expect(token.RBRACK, "]") {
			return elem
		}
		return &ast.ListComp{
			Elem:    elem,
			Clauses: clauses,
			Span:    ast.JoinSpan(openSpan.SpanOf(), spanPos(p.file, p.cur)),
		}
	}

	// It's a list literal - parse comma-separated elements
	elems := []ast.Expr{elem}

	for p.cur.Tok != token.RBRACK && p.cur.Tok != token.EOF {
		if p.cur.Tok != token.COMMA {
			break
		}
		p.next()    // consume ','
		p.skipNLs() // Allow newlines after comma
		if p.cur.Tok == token.RBRACK {
			break // allow trailing comma
		}
		nextElem := p.parseExpr()
		elems = append(elems, nextElem)
	}

	p.skipNLs() // Allow newlines before closing bracket
	end := spanPos(p.file, p.cur)
	if !p.expect(token.RBRACK, "]") {
		return &ast.Ident{Name: "<error>", Span: end}
	}

	return &ast.ListLit{
		Elems: elems,
		Span:  ast.JoinSpan(openSpan.SpanOf(), end),
	}
}

func (p *Parser) parseDictComp(openSpan ast.Node) ast.Expr {
	// Called with current token after '{'.
	key := p.parseExpr()
	if !p.expect(token.COLON, ":") {
		// recover to '}'
		for p.cur.Tok != token.RBRACE && p.cur.Tok != token.EOF {
			p.next()
		}
		_ = p.accept(token.RBRACE)
		return key
	}
	val := p.parseExpr()

	if !p.accept(token.KW_for) {
		p.errExpected(spanPos(p.file, p.cur), "'for' in dict comprehension")
		for p.cur.Tok != token.RBRACE && p.cur.Tok != token.EOF {
			p.next()
		}
		_ = p.accept(token.RBRACE)
		return key
	}

	clauses := p.parseCompClausesAfterFor()
	if !p.expect(token.RBRACE, "}") {
		return key
	}
	return &ast.DictComp{
		Key:     key,
		Val:     val,
		Clauses: clauses,
		Span:    ast.JoinSpan(openSpan.SpanOf(), spanPos(p.file, p.cur)),
	}
}

func (p *Parser) parseSetComp(openSpan ast.Node) ast.Expr {
	// Called with current token after '#{' sequence (we consumed both).
	// This handles both:
	//   - Set literals: #{1, 2, 3}
	//   - Set comprehensions: #{x for x in y}

	// Handle empty set: #{}
	if p.cur.Tok == token.RBRACE {
		end := spanPos(p.file, p.cur)
		p.next()
		return &ast.SetLit{
			Elems: nil,
			Span:  ast.JoinSpan(openSpan.SpanOf(), end),
		}
	}

	elem := p.parseExpr()

	// Check if it's a comprehension (has 'for')
	if p.cur.Tok == token.KW_for {
		p.next() // consume 'for'
		clauses := p.parseCompClausesAfterFor()
		if !p.expect(token.RBRACE, "}") {
			return elem
		}
		return &ast.SetComp{
			Elem:    elem,
			Clauses: clauses,
			Span:    ast.JoinSpan(openSpan.SpanOf(), spanPos(p.file, p.cur)),
		}
	}

	// It's a set literal - parse comma-separated elements
	elems := []ast.Expr{elem}

	for p.cur.Tok != token.RBRACE && p.cur.Tok != token.EOF {
		if p.cur.Tok != token.COMMA {
			break
		}
		p.next() // consume ','
		if p.cur.Tok == token.RBRACE {
			break // allow trailing comma
		}
		nextElem := p.parseExpr()
		elems = append(elems, nextElem)
	}

	end := spanPos(p.file, p.cur)
	if p.cur.Tok == token.RBRACE {
		p.next()
	}

	return &ast.SetLit{
		Elems: elems,
		Span:  ast.JoinSpan(openSpan.SpanOf(), end),
	}
}

// parseCompClausesAfterFor parses one or more "for … in … [if …]" chains,
// starting with the *current* token positioned *after* the initial 'for'.
func (p *Parser) parseCompClausesAfterFor() []ast.CompClause {
	var clauses []ast.CompClause

	for {
		// Parse <target> with membership disabled so KW_in is not eaten as a
		// binary operator here.
		var target ast.Expr
		withInAsOperator(false, func() {
			target = p.parseExpr()
		})

		if !p.expect(token.KW_in, "in") {
			// Bail out; caller will already be in an error-recovery path.
			return clauses
		}

		// Parse <iter> with normal membership behavior, but without ternary support
		// so 'if' is recognized as filter clause, not ternary operator
		iter := p.parseExprNoTernary()

		// Optional chain of `if <cond>`; combine multiple guards with `and`
		// so we keep a single If expression in the AST clause shape.
		var guard ast.Expr
		for p.cur.Tok == token.KW_if {
			p.next()
			g := p.parseExpr()
			if guard == nil {
				guard = g
			} else {
				guard = &ast.BinaryExpr{Op: "and", Lhs: guard, Rhs: g, Span: g.SpanOf()}
			}
		}

		clauses = append(clauses, ast.CompClause{
			Target: target,
			Iter:   iter,
			If:     guard,
		})

		// Support chained comprehensions: `... for ... in ... if ... for ...`
		if p.cur.Tok != token.KW_for {
			break
		}
		p.next() // consume the next `for` and loop again
	}

	return clauses
}
