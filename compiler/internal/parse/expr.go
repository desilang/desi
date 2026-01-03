package parse

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/token"
)

// ...

func (p *Parser) parseExpr() ast.Expr {
	return p.parseOr()
}

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

		case token.KW_is:
			// 'is' operator: x is pattern OR x is not pattern
			opStart := p.cur
			p.next()

			// Check for 'is not' (negated form)
			negated := false
			if p.cur.Tok == token.KW_not {
				negated = true
				p.next()
			}

			// Parse the pattern (could be: None, Some(x), ident, expr)
			pattern := p.parseBitOr()
			e = &ast.IsExpr{
				X:       e,
				Pattern: pattern,
				Negated: negated,
				Span:    joinTok(p.file, opStart, p.cur),
			}

		case token.KW_as:
			// 'as' operator: expr as type (explicit type cast)
			opStart := p.cur
			p.next()

			// Parse the target type
			targetType := p.parseTypeName()
			e = &ast.CastExpr{
				X:    e,
				Type: targetType,
				Span: joinTok(p.file, opStart, p.cur),
			}

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
	e := p.parseBitAnd()
	for p.cur.Tok == token.XOR {
		op := p.cur
		p.next()
		r := p.parseBitAnd()
		e = &ast.BinaryExpr{Op: "^", Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
	}
	return e
}

func (p *Parser) parseBitAnd() ast.Expr {
	e := p.parseShift()
	for p.cur.Tok == token.AMP {
		op := p.cur
		p.next()
		r := p.parseShift()
		e = &ast.BinaryExpr{Op: "&", Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
	}
	return e
}

func (p *Parser) parseShift() ast.Expr {
	e := p.parseAdd()
	for p.cur.Tok == token.LSHIFT || p.cur.Tok == token.RSHIFT {
		op := p.cur
		p.next()
		r := p.parseAdd()
		e = &ast.BinaryExpr{Op: op.Lexeme, Lhs: e, Rhs: r, Span: joinTok(p.file, op, p.cur)}
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
	case token.PLUS:
		op := p.cur
		p.next()
		x := p.parseUnary()
		return &ast.UnaryExpr{Op: "+", X: x, Span: joinTok(p.file, op, p.cur)}
	case token.BANG:
		op := p.cur
		p.next()
		x := p.parseUnary()
		return &ast.UnaryExpr{Op: "!", X: x, Span: joinTok(p.file, op, p.cur)}
	case token.TILDE:
		op := p.cur
		p.next()
		x := p.parseUnary()
		return &ast.UnaryExpr{Op: "~", X: x, Span: joinTok(p.file, op, p.cur)}
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

			// Check for comma - multi-index (e.g. Result[T, E])
			if p.cur.Tok == token.COMMA {
				// We have multiple indices - treat as a tuple
				elems := []ast.Expr{i1}
				for p.accept(token.COMMA) {
					elems = append(elems, p.parseExpr())
				}
				// Create implicit TupleLit for the index
				i1 = &ast.TupleLit{
					Elems: elems,
					Span:  ast.JoinSpan(i1.SpanOf(), spanPos(p.file, p.cur)),
				}
				// Ensure we expect closing bracket
				p.expect(token.RBRACK, "]")

				e = &ast.IndexExpr{
					X:    e,
					Idx:  i1,
					Span: ast.JoinSpan(idxStart, spanPos(p.file, p.cur)),
				}
				continue
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
			p.expect(token.RBRACK, "]")

			if isSlice {
				e = &ast.SliceExpr{
					X:    e,
					I:    i1,
					J:    i2,
					K:    i3,
					Span: ast.JoinSpan(idxStart, spanPos(p.file, p.cur)),
				}
			} else {
				e = &ast.IndexExpr{
					X:    e,
					Idx:  i1,
					Span: ast.JoinSpan(idxStart, spanPos(p.file, p.cur)),
				}
			}
		case token.DOT:
			dot := p.cur
			p.next()
			// Support both .name (field access) and .0/.1 (tuple index)
			if p.cur.Tok == token.INT_DEC {
				// Tuple index access: t.0, t.1, etc.
				idx := ast.Ident{Name: p.cur.Lexeme, Span: joinTok(p.file, dot, p.cur)}
				e = &ast.FieldExpr{X: e, Name: idx, Span: ast.JoinSpan(e.SpanOf(), spanPos(p.file, p.cur))}
				p.next()
			} else if p.cur.Tok != token.IDENT {
				p.errExpected(spanPos(p.file, p.cur), "identifier or integer index")
				return e
			} else {
				name := ast.Ident{Name: p.cur.Lexeme, Span: joinTok(p.file, dot, p.cur)}
				e = &ast.FieldExpr{X: e, Name: name, Span: ast.JoinSpan(e.SpanOf(), spanPos(p.file, p.cur))}
				p.next()
			}

		case token.QUESTION:
			// Postfix ? operator for error propagation (Result/Option)
			qSpan := spanPos(p.file, p.cur)
			p.next()
			e = &ast.TryExpr{X: e, Span: ast.JoinSpan(e.SpanOf(), qSpan)}

		case token.FLOAT:
			// Handle tuple index access: scanner emits ".0" as FLOAT, but after
			// an expression it should be treated as tuple index access (t.0)
			lex := p.cur.Lexeme
			if len(lex) > 1 && lex[0] == '.' {
				// Check if the rest is all digits
				suffix := lex[1:]
				allDigits := true
				for _, c := range suffix {
					if c < '0' || c > '9' {
						allDigits = false
						break
					}
				}
				if allDigits {
					// Treat as tuple index access
					idx := ast.Ident{Name: suffix, Span: spanPos(p.file, p.cur)}
					e = &ast.FieldExpr{X: e, Name: idx, Span: ast.JoinSpan(e.SpanOf(), spanPos(p.file, p.cur))}
					p.next()
					continue
				}
			}
			// Not a tuple index, done with postfix
			return e

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

	case token.DECIMAL_LIT:
		it := &ast.DecimalLit{Text: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
		p.next()
		return it

	case token.STR, token.LONGSTR:
		st := &ast.StrLit{
			Long: p.cur.Tok == token.LONGSTR,
			Span: spanPos(p.file, p.cur),
		}
		p.next()
		return st

	case token.RAWSTR:
		// Raw strings: r"...", r#"..."#, etc. - no escape processing
		st := &ast.StrLit{
			Long:  false,
			Raw:   true,
			Value: p.cur.Lexeme, // Raw content directly from lexer
			Span:  spanPos(p.file, p.cur),
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

	case token.KW_lambda:
		// Python-style: lambda x: expr  OR  lambda x, y: expr  OR  lambda x: int, y: int: expr
		return p.parseLambdaKeyword()

	case token.KW_match:
		return p.parseMatch()

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
		// back to classic parenthesized expression or tuple.
		if p.parenLambdaAhead() {
			return p.parseLambdaFromParen()
		}
		open := spanPos(p.file, p.cur)
		p.next() // consume '('

		// Handle empty tuple ()
		// TODO: Decide if () is unit/void or empty tuple. For now, treat as empty tuple.
		/*
			if p.cur.Tok == token.RPAREN {
				end := spanPos(p.file, p.cur)
				p.next()
				return &ast.TupleLit{Elems: nil, Span: ast.JoinSpan(open, end)}
			}
		*/

		// Detect named tuple: (name: value, ...)
		// Check if first element is IDENT followed by COLON (not type annotation for lambda)
		isNamed := false
		if p.cur.Tok == token.IDENT && p.peek.Tok == token.COLON {
			// Could be named tuple - look ahead to verify it's not a lambda param
			// In named tuple: (x: 10, y: 20) - after IDENT:, we get an expression
			// Not a lambda because parenLambdaAhead() already returned false
			isNamed = true
		}

		if isNamed {
			// Parse named tuple: (name: value, ...)
			var names []string
			var elems []ast.Expr
			for p.cur.Tok != token.RPAREN && p.cur.Tok != token.EOF {
				if p.cur.Tok != token.IDENT {
					p.errExpected(spanPos(p.file, p.cur), "field name")
					break
				}
				name := p.cur.Lexeme
				p.next() // consume name

				if !p.expect(token.COLON, ":") {
					break
				}

				value := p.parseExpr()
				names = append(names, name)
				elems = append(elems, value)

				if p.cur.Tok == token.COMMA {
					p.next()
				} else {
					break
				}
			}
			end := spanPos(p.file, p.cur)
			p.expectClose(token.RPAREN, ")", open)
			return &ast.TupleLit{Elems: elems, Names: names, Span: ast.JoinSpan(open, end)}
		}

		// Regular expression or positional tuple
		// Check for spread: *expr
		var e ast.Expr
		if p.cur.Tok == token.STAR {
			starSpan := spanPos(p.file, p.cur)
			p.next() // consume *
			inner := p.parseExpr()
			e = &ast.SpreadExpr{X: inner, Span: ast.JoinSpan(starSpan, inner.SpanOf())}
		} else {
			e = p.parseExpr()
		}

		// Check for comma to distinguish tuple from paren expr
		if p.cur.Tok == token.COMMA {
			p.next() // consume ','

			// Single element tuple (e,)
			if p.cur.Tok == token.RPAREN {
				end := spanPos(p.file, p.cur)
				p.next()
				return &ast.TupleLit{Elems: []ast.Expr{e}, Span: ast.JoinSpan(open, end)}
			}

			// Multi-element tuple (e1, e2, ...)
			elems := []ast.Expr{e}
			for p.cur.Tok != token.RPAREN && p.cur.Tok != token.EOF {
				// Check for spread: *expr
				var elem ast.Expr
				if p.cur.Tok == token.STAR {
					starSpan := spanPos(p.file, p.cur)
					p.next() // consume *
					inner := p.parseExpr()
					elem = &ast.SpreadExpr{X: inner, Span: ast.JoinSpan(starSpan, inner.SpanOf())}
				} else {
					elem = p.parseExpr()
				}
				elems = append(elems, elem)
				if p.cur.Tok == token.COMMA {
					p.next()
				} else {
					break
				}
			}
			end := spanPos(p.file, p.cur)
			p.expectClose(token.RPAREN, ")", open)
			return &ast.TupleLit{Elems: elems, Span: ast.JoinSpan(open, end)}
		}

		p.expectClose(token.RPAREN, ")", open)
		return e

	case token.LBRACK:
		open := spanPos(p.file, p.cur)
		p.next()
		return p.parseListLiteralOrComp(&ast.Ident{Name: "", Span: open}) // Span carrier

	case token.LBRACE:
		open := spanPos(p.file, p.cur)
		p.next()
		return p.parseDictLiteralOrComp(open)

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
			exprStart := spanPos(p.file, p.cur)
			p.next() // consume {
			expr := p.parseExpr()

			// Check for format spec: {expr:spec}
			var spec string
			if p.cur.Tok == token.COLON {
				p.next() // consume :
				// Parse format spec - collect characters until }
				// The spec can contain: fill, align, sign, #, 0, width, .precision, type
				spec = p.parseFStringSpec()
			}

			// Wrap in FStringExpr if there's a spec, otherwise use raw expr
			var part ast.Expr
			if spec != "" {
				part = &ast.FStringExpr{
					X:    expr,
					Spec: spec,
					Span: ast.JoinSpan(exprStart, spanPos(p.file, p.cur)),
				}
			} else {
				part = expr
			}
			parts = append(parts, part)
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

// parseFStringSpec parses a format specification after ':' in f-string.
// Collects characters until '}' is encountered.
// Grammar: [[fill]align][sign]['#']['0'][width]['.' precision][type]
// Examples: ".2f", "05d", ">10s", "x", "#x"
func (p *Parser) parseFStringSpec() string {
	var spec strings.Builder
	// Collect tokens until we hit RBRACE
	// The lexer may give us different token types for parts of the spec
	for p.cur.Tok != token.RBRACE && p.cur.Tok != token.EOF && p.cur.Tok != token.FSTR_END {
		spec.WriteString(p.cur.Lexeme)
		p.next()
	}
	return spec.String()
}

// parseDictLiteralOrComp disambiguates between dict literals and comprehensions.
// Called with current token after '{'.
func (p *Parser) parseDictLiteralOrComp(open diag.Span) ast.Expr {
	// Handle empty dict
	if p.cur.Tok == token.RBRACE {
		end := spanPos(p.file, p.cur)
		p.next() // consume '}'
		return &ast.DictLit{
			Keys:   nil,
			Values: nil,
			Span:   ast.JoinSpan(open, end),
		}
	}

	// Parse first key
	key := p.parseExpr()

	// Expect ':'
	if !p.expect(token.COLON, ":") {
		// Error recovery
		for p.cur.Tok != token.RBRACE && p.cur.Tok != token.EOF {
			p.next()
		}
		_ = p.accept(token.RBRACE)
		return key
	}

	// Parse first value
	val := p.parseExpr()

	// Check next token to disambiguate
	if p.cur.Tok == token.KW_for {
		// It's a dict comprehension - hand off to parseDictComp
		// We already parsed key and val, now parse the rest
		p.next() // consume 'for'
		clauses := p.parseCompClausesAfterFor()
		if !p.expect(token.RBRACE, "}") {
			return key
		}
		return &ast.DictComp{
			Key:     key,
			Val:     val,
			Clauses: clauses,
			Span:    ast.JoinSpan(open, spanPos(p.file, p.cur)),
		}
	}

	// It's a dict literal - continue parsing key-value pairs
	keys := []ast.Expr{key}
	values := []ast.Expr{val}

	for p.cur.Tok != token.RBRACE && p.cur.Tok != token.EOF {
		// Expect comma
		if p.cur.Tok != token.COMMA {
			break
		}
		p.next() // consume ','

		// Allow trailing comma
		if p.cur.Tok == token.RBRACE {
			break
		}

		// Parse next key
		nextKey := p.parseExpr()
		keys = append(keys, nextKey)

		// Expect ':'
		if !p.expect(token.COLON, ":") {
			break
		}

		// Parse next value
		nextVal := p.parseExpr()
		values = append(values, nextVal)
	}

	end := spanPos(p.file, p.cur)
	if p.cur.Tok == token.RBRACE {
		p.next() // consume '}'
	}

	return &ast.DictLit{
		Keys:   keys,
		Values: values,
		Span:   ast.JoinSpan(open, end),
	}
}

// parseDictLiteral parses a dict literal: {k1: v1, k2: v2, ...} or empty {}
// open is the Span of the opening '{'.
// NOTE: This is now only called from parseDictLiteralOrComp for the literal path.
func (p *Parser) parseDictLiteral(open diag.Span) ast.Expr {
	// Handle empty dict
	if p.cur.Tok == token.RBRACE {
		end := spanPos(p.file, p.cur)
		p.next() // consume '}'
		return &ast.DictLit{
			Keys:   nil,
			Values: nil,
			Span:   ast.JoinSpan(open, end),
		}
	}

	var keys []ast.Expr
	var values []ast.Expr

	for p.cur.Tok != token.RBRACE && p.cur.Tok != token.EOF {
		// Parse key expression
		key := p.parseExpr()
		keys = append(keys, key)

		// Expect ':'
		if !p.expect(token.COLON, ":") {
			// On error, try to continue parsing
			break
		}

		// Parse value expression
		val := p.parseExpr()
		values = append(values, val)

		// Check for comma or closing brace
		if p.cur.Tok == token.RBRACE {
			break
		}
		if p.cur.Tok == token.COMMA {
			p.next()
			// Allow trailing comma
			if p.cur.Tok == token.RBRACE {
				break
			}
			continue
		}

		// No comma and not '}' - error, but try to continue
		p.errUnexpected(spanPos(p.file, p.cur), "',' or '}'")
		break
	}

	end := spanPos(p.file, p.cur)
	if p.cur.Tok == token.RBRACE {
		p.next() // consume '}'
	}

	return &ast.DictLit{
		Keys:   keys,
		Values: values,
		Span:   ast.JoinSpan(open, end),
	}
}
