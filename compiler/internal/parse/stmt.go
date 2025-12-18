package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseBlock parses an indented block. It tolerates blank lines, converts an
// initial long string expression into a DocStringStmt, and joins span from
// first to last statement (or to current if empty).
func (p *Parser) parseBlock() *ast.Block {
	start := spanPos(p.file, p.cur)

	p.skipNLs()
	if !p.expect(token.Indent, "indent") {
		return &ast.Block{Span: start}
	}

	var stmts []ast.Stmt
	for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
		if p.cur.Tok == token.NL {
			p.next()
			continue
		}
		if s := p.parseStmt(); s != nil {
			stmts = append(stmts, s)
		} else {
			p.syncStmt()
		}
	}
	_ = p.expect(token.Dedent, "dedent")
	blk := &ast.Block{Stmts: stmts, Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}

	// Docstring: if the first statement is a *triple-quoted* string, convert to DocStringStmt.
	if len(blk.Stmts) > 0 {
		if es, ok := blk.Stmts[0].(*ast.ExprStmt); ok {
			if lit, ok2 := es.Expr.(*ast.StrLit); ok2 && lit.Long {
				blk.Stmts[0] = &ast.DocStringStmt{Value: lit, Span: es.SpanOf()}
			}
		}
	}

	return blk
}

// parseStmt dispatches statement forms. One-line forms (if/while/for) parse
// their SimpleStmt inline without requiring Indent/Dedent.
func (p *Parser) parseStmt() ast.Stmt {
	switch p.cur.Tok {
	case token.KW_let:
		return p.parseLet()
	case token.KW_return:
		return p.parseReturn()
	case token.KW_if:
		return p.parseIf()
	case token.KW_while:
		return p.parseWhile()
	case token.KW_for:
		return p.parseFor()
	case token.KW_using:
		return p.parseUsing()
	case token.KW_defer:
		return p.parseDefer()
	case token.KW_match:
		return p.parseMatch()
	case token.KW_import: // M5
		return p.parseImport()
	case token.KW_from: // M5
		return p.parseFromImport()
	case token.KW_unsafe: // M9C
		return p.parseUnsafe()
	default:
		// Parse the leading expression of a simple statement.
		e := p.parseExpr()

		// Friendly error for Python-style "name = expr" at statement start.
		// We only trigger this when the head expression is an identifier and
		// the very next token's lexeme is exactly "=" (which is not a token in Desi).
		if id, ok := e.(*ast.Ident); ok && p.cur.Lexeme == "=" && id != nil {
			// Point at the identifier span; then sync to end-of-statement.
			p.errMissingLetBeforeDecl(e.SpanOf())
			p.syncStmt()
			// Return a benign ExprStmt so the parser can continue.
			return &ast.ExprStmt{
				Expr: e,
				Span: ast.JoinSpan(e.SpanOf(), spanPos(p.file, p.cur)),
			}
		}

		if s := p.maybeMakeAssign(e); s != nil {
			return s
		}

		// Expression statement: require newline (tolerate EOF/Dedent).
		span := ast.JoinSpan(e.SpanOf(), spanPos(p.file, p.cur))

		if _, isMatch := e.(*ast.MatchExpr); isMatch {
			if p.cur.Tok == token.NL {
				p.next()
			}
		} else if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
			p.errExpected(spanPos(p.file, p.cur), "newline")
		}
		return &ast.ExprStmt{Expr: e, Span: span}
	}
}

// let [mut] name [: Type] = Expr
// let [mut] (name, name, ...) = Expr  (tuple destructuring)
func (p *Parser) parseLet() ast.Stmt {
	start := spanPos(p.file, p.cur)
	p.next() // 'let'

	mut := p.accept(token.KW_mut)

	// Check for tuple destructuring pattern: (a, b, c) or (a, *rest)
	var pattern []ast.Ident
	var name ast.Ident
	restIndex := -1 // -1 means no rest pattern

	if p.cur.Tok == token.LPAREN {
		// Tuple destructuring: let (a, b, c) = expr or let (a, *rest) = expr
		p.next() // consume '('
		patternIdx := 0
		for {
			// Check for rest pattern: *name
			if p.cur.Tok == token.STAR {
				if restIndex != -1 {
					p.errExpected(spanPos(p.file, p.cur), "only one rest pattern allowed")
					return nil
				}
				p.next() // consume '*'
				if p.cur.Tok != token.IDENT {
					p.errExpected(spanPos(p.file, p.cur), "identifier after *")
					return nil
				}
				restIndex = patternIdx
				pattern = append(pattern, ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)})
				p.next()
			} else if p.cur.Tok == token.IDENT {
				// Includes _ for ignore pattern
				pattern = append(pattern, ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)})
				p.next()
			} else {
				p.errExpected(spanPos(p.file, p.cur), "identifier or *rest")
				return nil
			}
			patternIdx++

			if p.cur.Tok == token.COMMA {
				p.next() // consume ','
				continue
			}
			break
		}
		if !p.expect(token.RPAREN, ")") {
			return nil
		}
	} else if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "identifier or (pattern)")
		return nil
	} else {
		name = ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
		p.next()
	}

	var ty *ast.TypeName
	if p.accept(token.COLON) {
		ty = p.parseTypeName()
	}

	if !p.expect(token.ASSIGN, "=") {
		return nil
	}
	val := p.parseExpr()

	// If the expression was a MatchExpr, it ends with a block (Dedent),
	// so we don't strictly need a newline separator before the next statement.
	if _, isMatch := val.(*ast.MatchExpr); isMatch {
		if p.cur.Tok == token.NL {
			p.next()
		}
	} else if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
		p.errExpected(spanPos(p.file, p.cur), "newline")
	}

	return &ast.LetStmt{
		Mutable:   mut,
		Name:      name,
		Pattern:   pattern,
		RestIndex: restIndex,
		Type:      ty,
		Value:     val,
		Span:      ast.JoinSpan(start, lastSpan(val, start)),
	}
}

// return [Expr]
func (p *Parser) parseReturn() ast.Stmt {
	start := spanPos(p.file, p.cur)
	p.next() // 'return'

	// Bare return (newline immediately)
	if p.cur.Tok == token.NL {
		p.next()
		return &ast.ReturnStmt{Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
	}

	// Otherwise parse a value.
	e := p.parseExpr()

	if _, isMatch := e.(*ast.MatchExpr); isMatch {
		if p.cur.Tok == token.NL {
			p.next()
		}
	} else if !p.accept(token.NL) && p.cur.Tok != token.EOF && p.cur.Tok != token.Dedent {
		p.errExpected(spanPos(p.file, p.cur), "newline")
	}
	return &ast.ReturnStmt{
		Value: e,
		Span:  ast.JoinSpan(start, lastSpan(e, start)),
	}
}
