package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// Expression precedence (highest to lowest):
//
//	** (right-assoc)
//	unary (- ! not await)
//	* / %
//	+ -
//	^
//	|           <-- inserted in M2 (bitwise OR)
//	< <= > >=
//	== !=
//	|>          (pipe)
//	and
//	or
//
// Notes:
//   - Postfix chain (call/index/field) remains greedy.
//   - StrLit.Long is set when the lexer produces LONGSTR ("""...""").
func (p *Parser) parseExpr() ast.Expr { return p.parseOr() }

func (p *Parser) parseOr() ast.Expr {
	e := p.parseAnd()
	for p.cur.Tok == token.KW_or {
		op := p.cur
		p.next()
		r := p.parseAnd()
		e = &ast.BinaryExpr{Op: "or", Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
	}
	return e
}

func (p *Parser) parseAnd() ast.Expr {
	e := p.parsePipe()
	for p.cur.Tok == token.KW_and {
		op := p.cur
		p.next()
		r := p.parsePipe()
		e = &ast.BinaryExpr{Op: "and", Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
	}
	return e
}

func (p *Parser) parsePipe() ast.Expr {
	e := p.parseEq()
	for p.cur.Tok == token.PIPE_GT {
		op := p.cur
		p.next()
		r := p.parseEq()
		e = &ast.BinaryExpr{Op: "|>", Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
	}
	return e
}

func (p *Parser) parseEq() ast.Expr {
	e := p.parseRel()
	for p.cur.Tok == token.EQEQ || p.cur.Tok == token.NEQ {
		op := p.cur
		p.next()
		r := p.parseRel()
		e = &ast.BinaryExpr{Op: op.Lexeme, Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
	}
	return e
}

func (p *Parser) parseRel() ast.Expr {
	e := p.parseBitOr()
	for {
		switch p.cur.Tok {
		case token.LT, token.LTE, token.GT, token.GTE:
			op := p.cur
			p.next()
			r := p.parseBitOr()
			e = &ast.BinaryExpr{Op: op.Lexeme, Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}

		case token.KW_in:
			// Membership: only when enabled (NOT inside a `for … in …` target).
			if !inAsOperator {
				return e
			}
			op := p.cur
			p.next()
			r := p.parseBitOr()
			e = &ast.BinaryExpr{Op: "in", Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}

		default:
			return e
		}
	}
}

func (p *Parser) parseBitOr() ast.Expr {
	e := p.parseXor()
	for p.cur.Tok == token.PIPE {
		op := p.cur
		p.next()
		r := p.parseXor()
		e = &ast.BinaryExpr{Op: "|", Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
	}
	return e
}

func (p *Parser) parseXor() ast.Expr {
	e := p.parseAdd()
	for p.cur.Tok == token.XOR {
		op := p.cur
		p.next()
		r := p.parseAdd()
		e = &ast.BinaryExpr{Op: "^", Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
	}
	return e
}

func (p *Parser) parseAdd() ast.Expr {
	e := p.parseMul()
	for p.cur.Tok == token.PLUS || p.cur.Tok == token.MINUS {
		op := p.cur
		p.next()
		r := p.parseMul()
		e = &ast.BinaryExpr{Op: op.Lexeme, Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
	}
	return e
}

func (p *Parser) parseMul() ast.Expr {
	e := p.parsePow()
	for p.cur.Tok == token.STAR || p.cur.Tok == token.SLASH || p.cur.Tok == token.PERCENT {
		op := p.cur
		p.next()
		r := p.parsePow()
		e = &ast.BinaryExpr{Op: op.Lexeme, Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
	}
	return e
}

func (p *Parser) parsePow() ast.Expr {
	left := p.parseUnary()
	if p.cur.Tok == token.POW {
		op := p.cur
		p.next()
		right := p.parsePow() // right-assoc
		return &ast.BinaryExpr{Op: "**", Lhs: left, Rhs: right, Span: joinTok(p.file, op, p.cur)}
	}
	return left
}

func (p *Parser) parseUnary() ast.Expr {
	switch p.cur.Tok {
	case token.MINUS:
		op := p.cur
		p.next()
		x := p.parseUnary()
		return &ast.UnaryExpr{Op: "-", X: x, Span: joinTok(p.file, op, p.cur)}
	case token.BANG:
		op := p.cur
		p.next()
		x := p.parseUnary()
		return &ast.UnaryExpr{Op: "!", X: x, Span: joinTok(p.file, op, p.cur)}
	case token.KW_not:
		op := p.cur
		p.next()
		x := p.parseUnary()
		return &ast.UnaryExpr{Op: "not", X: x, Span: joinTok(p.file, op, p.cur)}
	case token.KW_await:
		op := p.cur
		p.next()
		x := p.parseUnary()
		return &ast.UnaryExpr{Op: "await", X: x, Span: joinTok(p.file, op, p.cur)}
	default:
		return p.parsePostfix()
	}
}

func (p *Parser) parsePostfix() ast.Expr {
	e := p.parsePrimary()
	for {
		switch p.cur.Tok {
		case token.LPAREN:
			callStart := spanPos(p.file, p.cur)
			p.next()

			var args []ast.Expr        // legacy vector
			var argNodes []ast.CallArg // canonical vector (with names)

			if p.cur.Tok != token.RPAREN {
				for {
					// Named argument: IDENT '=' Expr  → CallArg{Name:&Ident, Expr:Expr}
					if p.cur.Tok == token.IDENT && p.peek.Tok == token.ASSIGN {
						nameTok := p.cur
						nameIdent := &ast.Ident{Name: nameTok.Lexeme, Span: spanPos(p.file, nameTok)}
						p.next()                        // name
						_ = p.expect(token.ASSIGN, "=") // consume '=' (with recovery)
						val := p.parseExpr()
						argNodes = append(argNodes, ast.CallArg{Name: nameIdent, Expr: val, Star: false})
						args = append(args, val) // keep legacy positional for now
					} else {
						// Plain positional
						val := p.parseExpr()
						argNodes = append(argNodes, ast.CallArg{Name: nil, Expr: val, Star: false})
						args = append(args, val)
					}

					if !p.accept(token.COMMA) {
						break
					}
					if p.cur.Tok == token.RPAREN {
						break
					}
				}
			}
			p.expectClose(token.RPAREN, ")", callStart)
			e = &ast.CallExpr{
				Callee:   e,
				Args:     args,
				ArgNodes: argNodes,
				Span:     ast.JoinSpan(callStart, spanPos(p.file, p.cur)),
			}

		case token.LBRACK:
			idxStart := spanPos(p.file, p.cur)
			p.next()

			// Detect slice forms by watching for ':' separators.
			// Grammar (all parts optional where shown):
			//   [i]              -> IndexExpr
			//   [i:j] [i:j:k] [:j] [i:] [:] [::k] [:j:k] [i::k]
			var i1, i2, i3 ast.Expr
			isSlice := false

			// First part (may be empty for [:...])
			if p.cur.Tok != token.COLON && p.cur.Tok != token.RBRACK {
				i1 = p.parseExpr()
			}
			// If we see a colon, it's a slice.
			if p.cur.Tok == token.COLON {
				isSlice = true
				p.next() // consume first ':'
				// Second part (stop) optional if next is ':' or ']'
				if p.cur.Tok != token.COLON && p.cur.Tok != token.RBRACK {
					i2 = p.parseExpr()
				}
				// Optional stride
				if p.cur.Tok == token.COLON {
					p.next()
					if p.cur.Tok != token.RBRACK {
						i3 = p.parseExpr()
					}
				}
			}
			p.expectClose(token.RBRACK, "]", idxStart)
			if isSlice {
				e = &ast.SliceExpr{X: e, I: i1, J: i2, K: i3, Span: ast.JoinSpan(idxStart, spanPos(p.file, p.cur))}
			} else {
				e = &ast.IndexExpr{X: e, Idx: i1, Span: ast.JoinSpan(idxStart, spanPos(p.file, p.cur))}
			}

		case token.DOT:
			dot := p.cur
			p.next()
			if p.cur.Tok != token.IDENT {
				p.errExpected(spanPos(p.file, p.cur), "identifier")
				return e
			}
			name := ast.Ident{Name: p.cur.Lexeme, Span: joinTok(p.file, dot, p.cur)}
			e = &ast.FieldExpr{X: e, Name: name, Span: ast.JoinSpan(e.SpanOf(), spanPos(p.file, p.cur))}
			p.next()

		default:
			return e
		}
	}
}

func (p *Parser) parsePrimary() ast.Expr {
	switch p.cur.Tok {
	case token.IDENT:
		// Lambda (single-ident) only when '=>' follows immediately.
		if p.peek.Tok == token.FAT_ARROW {
			return p.parseLambdaFromIdent()
		}
		id := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
		p.next()
		return &id

	case token.INT_DEC, token.INT_HEX, token.INT_BIN, token.INT_OCT:
		it := &ast.IntLit{Text: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
		p.next()
		return it

	case token.FLOAT, token.FLOAT_EXP:
		it := &ast.FloatLit{Text: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
		p.next()
		return it

	case token.STR, token.LONGSTR:
		st := &ast.StrLit{
			Long: p.cur.Tok == token.LONGSTR,
			Span: spanPos(p.file, p.cur),
		}
		p.next()
		return st

	case token.FSTR_START:
		return p.parseFString()

	case token.KW_true:
		b := &ast.BoolLit{Value: true, Span: spanPos(p.file, p.cur)}
		p.next()
		return b

	case token.KW_false:
		b := &ast.BoolLit{Value: false, Span: spanPos(p.file, p.cur)}
		p.next()
		return b

	case token.KW_none:
		n := &ast.NoneLit{Span: spanPos(p.file, p.cur)}
		p.next()
		return n

	case token.KW_async:
		// Allow: async (x, y) => expr   |   async x => expr
		as := spanPos(p.file, p.cur)
		p.next() // consume 'async'

		// Parenthesized lambda head: ( ... ) => ...
		if p.cur.Tok == token.LPAREN && p.parenLambdaAhead() {
			e := p.parseLambdaFromParen()
			if lam, ok := e.(*ast.LambdaExpr); ok {
				lam.Async = true
				lam.Span = ast.JoinSpan(as, lam.Span)
			}
			return e
		}

		// Single-ident lambda head: ident => expr
		if p.cur.Tok == token.IDENT && p.peek.Tok == token.FAT_ARROW {
			e := p.parseLambdaFromIdent()
			if lam, ok := e.(*ast.LambdaExpr); ok {
				lam.Async = true
				lam.Span = ast.JoinSpan(as, lam.Span)
			}
			return e
		}

		// Otherwise, 'async' isn't valid in expression position here.
		p.errUnexpected(as, "expression")
		errId := &ast.Ident{Name: "<error>", Span: as}
		return errId

	case token.LPAREN:
		// If this '(' starts a parenthesized lambda head whose matching ')'
		// is immediately followed by '=>', parse a lambda; otherwise fall
		// back to classic parenthesized expression.
		if p.parenLambdaAhead() {
			return p.parseLambdaFromParen()
		}
		open := spanPos(p.file, p.cur)
		p.next()
		e := p.parseExpr()
		p.expectClose(token.RPAREN, ")", open)
		return e

	case token.LBRACK:
		open := spanPos(p.file, p.cur)
		p.next()
		return p.parseListComp(&ast.Ident{Name: "", Span: open}) // Span carrier

	case token.LBRACE:
		open := spanPos(p.file, p.cur)
		p.next()
		return p.parseDictComp(&ast.Ident{Name: "", Span: open})

	case token.HASH:
		// Set comprehension starts with "#{".
		hash := spanPos(p.file, p.cur)
		p.next()
		if !p.expect(token.LBRACE, "{") {
			return &ast.Ident{Name: "<error>", Span: hash}
		}
		return p.parseSetComp(&ast.Ident{Name: "", Span: hash})

	default:
		p.errUnexpected(spanPos(p.file, p.cur), "expression")
		errId := &ast.Ident{Name: "<error>", Span: spanPos(p.file, p.cur)}
		if p.cur.Tok != token.EOF {
			p.next()
		}
		return errId
	}
}

func (p *Parser) parseFString() ast.Expr {
	start := spanPos(p.file, p.cur)
	p.next() // consume FSTR_START

	var parts []ast.Expr
	for {
		switch p.cur.Tok {
		case token.FSTR_PART:
			st := &ast.StrLit{
				Long:  false,
				Value: p.cur.Lexeme, // Store the literal text from the token
				Span:  spanPos(p.file, p.cur),
			}
			parts = append(parts, st)
			p.next()

		case token.LBRACE:
			p.next() // consume {
			expr := p.parseExpr()
			parts = append(parts, expr)
			if !p.expect(token.RBRACE, "}") {
				return &ast.FString{Parts: parts, Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
			}

		case token.FSTR_END:
			end := spanPos(p.file, p.cur)
			p.next() // consume "
			return &ast.FString{Parts: parts, Span: ast.JoinSpan(start, end)}

		default:
			p.errUnexpected(spanPos(p.file, p.cur), "f-string part or end")
			return &ast.FString{Parts: parts, Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
		}
	}
}
