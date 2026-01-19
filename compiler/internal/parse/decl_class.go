package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseClassWithDecs parses a class declaration, using any pre-parsed decorators.
// isNested controls default visibility: top-level classes are public by default.
func (p *Parser) parseClassWithDecs(decs []*ast.Decorator, isNested bool) *ast.ClassDecl {
	start := spanPos(p.file, p.cur)
	if len(decs) > 0 {
		start = decs[0].Span
	}

	// Optional 'pub' before 'class'
	explicitPub := p.accept(token.KW_pub)

	if !p.expect(token.KW_class, "class") {
		return nil
	}
	if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "class name")
		return nil
	}
	name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
	p.next()

	// Parse optional type parameters: <T> or <T, U> or <T: Trait>
	var typeParams []*ast.TypeParamNode
	if p.cur.Tok == token.LT { // <
		p.next()
		for {
			if p.cur.Tok != token.IDENT {
				p.errExpected(spanPos(p.file, p.cur), "type parameter name")
				break
			}
			tpStart := spanPos(p.file, p.cur)
			tpName := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
			p.next()

			var bounds []*ast.Ident
			if p.accept(token.COLON) {
				for {
					if p.cur.Tok != token.IDENT {
						p.errExpected(spanPos(p.file, p.cur), "trait name")
						break
					}
					bounds = append(bounds, &ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)})
					p.next()
					if !p.accept(token.PLUS) {
						break
					}
				}
			}

			typeParams = append(typeParams, &ast.TypeParamNode{
				Name:   tpName,
				Bounds: bounds,
				Span:   ast.JoinSpan(tpStart, spanPos(p.file, p.cur)),
			})

			if p.cur.Tok == token.GT { // >
				p.next()
				break
			}
			if !p.expect(token.COMMA, ",") {
				break
			}
		}
	}

	// Optional base list: "(" BaseList ")"
	var bases []*ast.TypeName
	if p.accept(token.LPAREN) {
		open := spanPos(p.file, p.cur)
		if p.cur.Tok != token.RPAREN {
			for {
				bases = append(bases, p.parseTypeName())
				if !p.accept(token.COMMA) {
					break
				}
				// allow trailing comma
				if p.cur.Tok == token.RPAREN {
					break
				}
			}
		}
		p.expectClose(token.RPAREN, ")", open)
	}

	if !p.expect(token.COLON, ":") {
		p.syncStmt()
		return nil
	}
	if !p.expect(token.NL, "newline") {
		p.syncStmt()
		return nil
	}

	// Class body
	// Skip any NL tokens from comment-only lines before the body starts
	p.skipNLs()
	bodyStart := spanPos(p.file, p.cur)
	if !p.expect(token.Indent, "indent") {
		return nil
	}

	var (
		fields       []*ast.FieldDecl
		methods      []*ast.FuncDecl
		nested       []*ast.ClassDecl
		constants    []*ast.ClassConstDecl
		staticFields []*ast.ClassStaticDecl
		doc          *ast.StrLit
	)

	for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
		p.skipNLs()
		if p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
			break
		}

		// Handle unexpected Indent (from failed parsing that left nested blocks)
		if p.cur.Tok == token.Indent {
			// Skip entire nested block
			depth := 1
			p.next()
			for depth > 0 && p.cur.Tok != token.EOF {
				if p.cur.Tok == token.Indent {
					depth++
				} else if p.cur.Tok == token.Dedent {
					depth--
				}
				p.next()
			}
			continue
		}

		// Leading docstring in the body attaches to the class and is removed.
		if doc == nil && p.cur.Tok == token.LONGSTR {
			doc = &ast.StrLit{Long: true, Span: spanPos(p.file, p.cur)}
			p.next()
			_ = p.accept(token.NL) // tolerate missing NL after long string
			continue
		}

		// Zero or more decorators on the member.
		memDecs := p.parseDecorators()
		p.skipNLs() // ensure we're at the member header token

		// Classify member WITHOUT consuming tokens like 'pub' prematurely.
		tok := p.cur.Tok
		peek := p.peek.Tok

		// Nested class?
		if tok == token.KW_class || (tok == token.KW_pub && peek == token.KW_class) {
			if c := p.parseClassWithDecs(memDecs, true /*nested*/); c != nil {
				nested = append(nested, c)
			}
			continue
		}

		// Method?
		if tok == token.KW_def || tok == token.KW_async ||
			(tok == token.KW_pub && (peek == token.KW_def || peek == token.KW_async)) {
			if m := p.parseMethodWithDecs(memDecs); m != nil {
				methods = append(methods, m)
			}
			continue
		}

		// Otherwise: Field, Constant, or Static Field
		// Check for 'pub' modifier
		isPub := false
		if p.cur.Tok == token.KW_pub {
			// Check what follows 'pub'
			if p.peek.Tok == token.KW_const ||
				p.peek.Tok == token.KW_static ||
				p.peek.Tok == token.KW_mut {
				isPub = true
				p.next()
			} else if p.peek.Tok == token.IDENT {
				// Could be a field (name: type) - commit to field parsing
				isPub = true
				p.next()
			} else {
				// Invalid token after 'pub' (not a recognized pattern)
				p.errExpected(spanPos(p.file, p.peek), "def, field declaration, const, static, or mut")
				// Skip until we find a safe recovery point
				for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
					if p.cur.Tok == token.Indent {
						// Skip nested block entirely
						depth := 1
						p.next()
						for depth > 0 && p.cur.Tok != token.EOF {
							if p.cur.Tok == token.Indent {
								depth++
							} else if p.cur.Tok == token.Dedent {
								depth--
							}
							p.next()
						}
					} else if p.cur.Tok == token.NL {
						p.next()
						break
					} else {
						p.next()
					}
				}
				continue
			}
		}

		// Check for 'mut' modifier (for mutable fields or static fields)
		isMut := false
		if p.cur.Tok == token.KW_mut {
			// mut can precede: static (for static fields) or IDENT (for regular mutable fields)
			if p.peek.Tok == token.KW_static || p.peek.Tok == token.IDENT {
				isMut = true
				p.next() // consume 'mut'
			}
		}

		// Check for 'static' (Class Static Field)
		if p.cur.Tok == token.KW_static {
			p.next() // consume 'static'
			// Expect identifier
			if p.cur.Tok != token.IDENT {
				p.errExpected(spanPos(p.file, p.cur), "static field name")
				p.syncStmt()
				continue
			}
			name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
			p.next()

			// Expect type annotation ": Type"
			if !p.expect(token.COLON, ":") {
				p.syncStmt()
				continue
			}
			typ := p.parseTypeName()

			// Expect value assignment "= Value"
			if !p.expect(token.ASSIGN, "=") {
				p.syncStmt()
				continue
			}
			val := p.parseExpr()

			staticDecl := &ast.ClassStaticDecl{
				Name:  name,
				Type:  typ,
				Value: val,
				IsPub: isPub,
				IsMut: isMut,
				Span:  ast.JoinSpan(name.Span, val.SpanOf()),
			}
			staticFields = append(staticFields, staticDecl)
			if !p.accept(token.NL) && p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
				p.errExpected(spanPos(p.file, p.cur), "newline")
			}
			continue
		}

		// Check for 'const' (Class Constant)
		if p.cur.Tok == token.KW_const {
			p.next() // consume 'const'
			// Expect identifier
			if p.cur.Tok != token.IDENT {
				p.errExpected(spanPos(p.file, p.cur), "constant name")
				p.syncStmt()
				continue
			}
			name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
			p.next()

			// Expect type annotation ": Type"
			if !p.expect(token.COLON, ":") {
				p.syncStmt()
				continue
			}
			typ := p.parseTypeName()

			// Expect value assignment "= Value"
			if !p.expect(token.ASSIGN, "=") {
				p.syncStmt()
				continue
			}
			val := p.parseExpr()

			constDecl := &ast.ClassConstDecl{
				Name:  name,
				Type:  typ,
				Value: val,
				IsPub: isPub,
				Span:  ast.JoinSpan(name.Span, val.SpanOf()),
			}
			constants = append(constants, constDecl)
			if !p.accept(token.NL) && p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
				p.errExpected(spanPos(p.file, p.cur), "newline")
			}
			continue
		}

		// If not a constant, it must be a field.
		if p.cur.Tok != token.IDENT {
			p.errExpected(spanPos(p.file, p.cur), "field name")
			p.syncStmt()
			continue
		}
		fname := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
		p.next()
		if !p.expect(token.COLON, ":") {
			p.syncStmt()
			continue
		}
		ty := p.parseTypeName()
		if !p.accept(token.NL) && p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
			p.errExpected(spanPos(p.file, p.cur), "newline")
		}
		fields = append(fields, &ast.FieldDecl{
			Pub:  isPub,
			Mut:  isMut,
			Name: fname,
			Type: ty,
			Span: ast.JoinSpan(fname.Span, ty.Span),
		})
	}

	_ = p.expect(token.Dedent, "dedent")

	decl := &ast.ClassDecl{
		Pub:          explicitPub || !isNested, // top-level default public
		Name:         name,
		TypeParams:   typeParams,
		Bases:        bases,
		Fields:       fields,
		Methods:      methods,
		Constants:    constants,
		StaticFields: staticFields,
		Nested:       nested,
		Decorators:   decs,
		Doc:          doc,
		Span:         ast.JoinSpan(start, bodyStart),
	}
	return decl
}

// parseMethodWithDecs parses a class method (decorators already read if any).
func (p *Parser) parseMethodWithDecs(decs []*ast.Decorator) *ast.FuncDecl {
	start := spanPos(p.file, p.cur)
	if len(decs) > 0 {
		start = decs[0].Span
	}

	pub := p.accept(token.KW_pub)
	async := p.accept(token.KW_async)

	if !p.expect(token.KW_def, "def") {
		if async {
			p.errAsyncBeforeDef(start)
		}
		return nil
	}
	if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "method name")
		return nil
	}
	name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
	p.next()

	// Parse optional type parameters: <T> or <T, U> or <T: Trait>
	var typeParams []*ast.TypeParamNode
	if p.cur.Tok == token.LT { // <
		p.next()
		for {
			if p.cur.Tok != token.IDENT {
				p.errExpected(spanPos(p.file, p.cur), "type parameter name")
				break
			}
			tpStart := spanPos(p.file, p.cur)
			tpName := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
			p.next()

			var bounds []*ast.Ident
			if p.accept(token.COLON) {
				for {
					if p.cur.Tok != token.IDENT {
						p.errExpected(spanPos(p.file, p.cur), "trait name")
						break
					}
					bounds = append(bounds, &ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)})
					p.next()
					if !p.accept(token.PLUS) {
						break
					}
				}
			}

			typeParams = append(typeParams, &ast.TypeParamNode{
				Name:   tpName,
				Bounds: bounds,
				Span:   ast.JoinSpan(tpStart, spanPos(p.file, p.cur)),
			})

			if p.cur.Tok == token.GT { // >
				p.next()
				break
			}
			if !p.expect(token.COMMA, ",") {
				break
			}
		}
	}

	lparen := spanPos(p.file, p.cur)
	if !p.expect(token.LPAREN, "(") {
		return nil
	}
	var params []ast.Param
	if p.cur.Tok != token.RPAREN {
		params = p.parseParams()
	}
	p.expectClose(token.RPAREN, ")", lparen)

	var ret *ast.TypeName
	if p.accept(token.ARROW) {
		ret = p.parseTypeName()
	}

	// Body: ":" (NL Block | one-line simple stmt) | NL
	var body *ast.Block
	if p.accept(token.COLON) {
		if p.cur.Tok == token.NL {
			p.next()
			body = p.parseBlock()
		} else {
			s := p.parseSimpleStmtInline()
			body = &ast.Block{Stmts: []ast.Stmt{s}, Span: ast.JoinSpan(s.SpanOf(), s.SpanOf())}
		}
	} else {
		p.expect(token.NL, "newline")
	}

	fn := &ast.FuncDecl{
		Async:      async,
		Pub:        pub,
		Name:       name,
		TypeParams: typeParams,
		Params:     params,
		RetType:    ret,
		Body:       body,
		Decorators: decs,
		Span:       ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}

	// Attach leading docstring (triple-quoted) if present as the first stmt.
	if fn.Body != nil && len(fn.Body.Stmts) > 0 {
		if ds, ok := fn.Body.Stmts[0].(*ast.DocStringStmt); ok && ds.Value != nil && ds.Value.Long {
			fn.Doc = ds.Value
			fn.Body.Stmts = fn.Body.Stmts[1:]
		}
	}
	return fn
}
