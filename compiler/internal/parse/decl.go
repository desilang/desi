package parse

import (
	"bytes"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

func (p *Parser) parseFunc() *ast.FuncDecl { return p.parseFuncWithDecs(nil) }

func (p *Parser) parseFuncWithDecs(decs []*ast.Decorator) *ast.FuncDecl {
	// Optional decorators
	if decs == nil {
		decs = p.parseDecorators()
	}
	start := spanPos(p.file, p.cur)
	if len(decs) > 0 {
		start = decs[0].Span
	}

	// NEW: optional 'pub' before function headers (top-level + decorated)
	pub := p.accept(token.KW_pub)
	// Optional 'async' before 'def'
	async := p.accept(token.KW_async)

	if !p.expect(token.KW_def, "def") {
		if async {
			p.errAsyncBeforeDef(start)
		}
		return nil
	}

	if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "function name")
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

			// Parse optional trait bounds: T: Trait or T: Trait1 + Trait2
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
		Pub:        pub, // NEW: top-level pub now supported
		Name:       name,
		TypeParams: typeParams,
		Params:     params,
		RetType:    ret,
		Body:       body,
		Decorators: decs,
		Span:       ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}

	// Attach leading docstring if present.
	if fn.Body != nil && len(fn.Body.Stmts) > 0 {
		if ds, ok := fn.Body.Stmts[0].(*ast.DocStringStmt); ok && ds.Value != nil && ds.Value.Long {
			fn.Doc = ds.Value
			fn.Body.Stmts = fn.Body.Stmts[1:]
		}
	}
	return fn
}

func (p *Parser) parseParams() []ast.Param {
	var out []ast.Param
	seenVariadic := false

	for {
		// Optional leading parameter mode: 'ref' | 'inout' (soft keywords).
		paramMode := ast.ParamMove
		paramStart := spanPos(p.file, p.cur)

		if p.cur.Tok == token.KW_ref || p.cur.Tok == token.KW_inout ||
			(p.cur.Tok == token.IDENT && (p.cur.Lexeme == "ref" || p.cur.Lexeme == "inout")) {
			if p.cur.Tok == token.KW_ref || p.cur.Lexeme == "ref" {
				paramMode = ast.ParamRef
			} else {
				paramMode = ast.ParamInout
			}
			paramStart = spanPos(p.file, p.cur)
			p.next()
		}

		// Check for kwargs '**' prefix or variadic '*' prefix
		variadic := false
		kwargs := false
		if p.accept(token.POW) {
			// ** = kwargs (must check POW before STAR since ** is one token)
			kwargs = true
			if seenVariadic {
				p.errExpected(spanPos(p.file, p.cur), "only one variadic/**kwargs parameter allowed")
			}
			seenVariadic = true
		} else if p.accept(token.STAR) {
			variadic = true
			if seenVariadic {
				p.errExpected(spanPos(p.file, p.cur), "only one variadic/**kwargs parameter allowed")
			}
			seenVariadic = true
		}

		// Name is required.
		if p.cur.Tok != token.IDENT {
			p.errUnexpected(spanPos(p.file, p.cur), "parameter")
			break
		}
		name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
		p.next()

		// Optional ": Type"
		var ty *ast.TypeName
		if p.accept(token.COLON) {
			ty = p.parseTypeName()
		}

		// Optional "= defaultExpr"
		var def ast.Expr
		if p.accept(token.ASSIGN) {
			def = p.parseExpr()
		}

		if (variadic || kwargs) && def != nil {
			p.errExpected(def.SpanOf(), "variadic/**kwargs parameter cannot have default value")
		}

		out = append(out, ast.Param{
			Name:     name,
			Type:     ty,
			Default:  def,
			Mode:     paramMode,
			Variadic: variadic,
			Kwargs:   kwargs,
			Span:     ast.JoinSpan(paramStart, lastSpan(def, name.Span)),
		})

		// Comma or end.
		if !p.accept(token.COMMA) {
			break
		}
		if p.cur.Tok == token.RPAREN {
			break // trailing comma
		}
		if seenVariadic {
			p.errExpected(spanPos(p.file, p.cur), "variadic parameter must be last")
		}
	}
	return out
}

func (p *Parser) parseTypeName() *ast.TypeName {
	// Accept IDENT, 'none', '(' (tuple), or '[' (list-like meta) as type name start
	if p.cur.Tok != token.IDENT && p.cur.Tok != token.KW_none && p.cur.Tok != token.LPAREN && p.cur.Tok != token.LBRACK {
		p.errExpected(spanPos(p.file, p.cur), "type name")
		return &ast.TypeName{Name: "<?", Span: spanPos(p.file, p.cur)}
	}
	start := spanPos(p.file, p.cur)
	var b bytes.Buffer

	// Handle 'none' keyword
	if p.cur.Tok == token.KW_none {
		b.WriteString("none")
		p.next()
		// none doesn't have parameters or dots
		firstType := &ast.TypeName{
			Name: "none",
			Span: ast.JoinSpan(start, spanPos(p.file, p.cur)),
		}
		// Check for union continuation
		if p.cur.Tok != token.PIPE {
			return firstType
		}
		// Parse union
		variants := []*ast.TypeName{firstType}
		for p.accept(token.PIPE) {
			variant := p.parseTypeName()
			variants = append(variants, variant)
		}
		return &ast.TypeName{
			Name:       "",
			UnionTypes: variants,
			Span:       ast.JoinSpan(start, spanPos(p.file, p.cur)),
		}
	}

	// Handle Tuple types: (T1, T2, ...)
	if p.cur.Tok == token.LPAREN {
		p.next()
		var elems []*ast.TypeName
		for {
			elem := p.parseTypeName()
			elems = append(elems, elem)
			if !p.accept(token.COMMA) {
				break
			}
		}
		p.expect(token.RPAREN, ")")

		firstType := &ast.TypeName{
			Name:       "",
			TupleTypes: elems,
			Span:       ast.JoinSpan(start, spanPos(p.file, p.cur)),
		}

		// Check for union continuation
		if p.cur.Tok != token.PIPE {
			return firstType
		}
		// Parse union
		variants := []*ast.TypeName{firstType}
		for p.accept(token.PIPE) {
			variant := p.parseTypeName()
			variants = append(variants, variant)
		}
		return &ast.TypeName{
			Name:       "",
			UnionTypes: variants,
			Span:       ast.JoinSpan(start, spanPos(p.file, p.cur)),
		}
	}

	// Handle list-like type syntax: [item1, item2]
	// Used in Meta class fields: unique_together: [title, author_id]
	if p.cur.Tok == token.LBRACK {
		p.next()
		var items []*ast.TypeName
		if p.cur.Tok != token.RBRACK {
			for {
				item := p.parseTypeConstructorArg()
				items = append(items, item)
				if !p.accept(token.COMMA) {
					break
				}
			}
		}
		p.expect(token.RBRACK, "]")
		return &ast.TypeName{
			Name:   "",
			Params: items,
			Span:   ast.JoinSpan(start, spanPos(p.file, p.cur)),
		}
	}

	// Regular IDENT path
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

	// Parse type parameters: name[T1, T2] or name<T1, T2> or name(arg1, key=val)
	var params []*ast.TypeName
	var kwParams []ast.TypeNameKwArg
	if p.accept(token.LBRACK) {
		for {
			param := p.parseTypeName()
			params = append(params, param)

			if !p.accept(token.COMMA) {
				break
			}
		}
		p.expect(token.RBRACK, "]")
	} else if p.accept(token.LT) { // <
		for {
			param := p.parseTypeName()
			params = append(params, param)

			if !p.accept(token.COMMA) {
				break
			}
		}
		p.expectTypeGT() // Use expectTypeGT for nested generics like Box<Box<int>>
	} else if p.accept(token.LPAREN) {
		// Call-like params: CharField(100, unique=true), ForeignKey(User, on_delete=CASCADE)
		if p.cur.Tok != token.RPAREN {
			for {
				// Check for keyword arg: IDENT = value
				if p.cur.Tok == token.IDENT && p.peek.Tok == token.ASSIGN {
					key := p.cur.Lexeme
					p.next() // consume key
					p.next() // consume =
					// Value can be IDENT (CASCADE, true, false), INT, or a TypeName
					val := p.parseTypeConstructorArg()
					kwParams = append(kwParams, ast.TypeNameKwArg{Key: key, Value: val})
				} else {
					// Positional arg: can be IDENT (User, CASCADE), INT (100, 200), true/false
					param := p.parseTypeConstructorArg()
					params = append(params, param)
				}
				if !p.accept(token.COMMA) {
					break
				}
			}
		}
		p.expect(token.RPAREN, ")")
	}

	firstType := &ast.TypeName{
		Name:     b.String(),
		Params:   params,
		KwParams: kwParams,
		Span:     ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}

	// Parse union types: type1|type2|type3
	// Check for | token (PIPE)
	if p.cur.Tok != token.PIPE {
		return firstType
	}

	// We have a union type
	variants := []*ast.TypeName{firstType}
	for p.accept(token.PIPE) {
		variant := p.parseTypeName()
		variants = append(variants, variant)
	}

	// Return a TypeName with UnionTypes field
	return &ast.TypeName{
		Name:       "", // Union types don't have a single name
		UnionTypes: variants,
		Span:       ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}
}

// parseTypeConstructorArg parses a single argument in a type constructor call.
// Handles: IDENT (User, CASCADE), INT (100, 200), true/false/none keywords.
// Returns a TypeName where the Name field holds the string representation.
func (p *Parser) parseTypeConstructorArg() *ast.TypeName {
	start := spanPos(p.file, p.cur)
	switch p.cur.Tok {
	case token.INT_DEC:
		name := p.cur.Lexeme
		p.next()
		return &ast.TypeName{Name: name, Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
	case token.KW_true:
		p.next()
		return &ast.TypeName{Name: "true", Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
	case token.KW_false:
		p.next()
		return &ast.TypeName{Name: "false", Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
	case token.KW_none:
		p.next()
		return &ast.TypeName{Name: "none", Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
	case token.STR:
		// String literal: used for SQL expressions in GeneratedField, etc.
		name := p.cur.Lexeme
		p.next()
		return &ast.TypeName{Name: name, Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
	case token.MINUS:
		// Negative number prefix: -100, -created_at (ordering)
		p.next()
		if p.cur.Tok == token.INT_DEC {
			name := "-" + p.cur.Lexeme
			p.next()
			return &ast.TypeName{Name: name, Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
		}
		if p.cur.Tok == token.IDENT {
			name := "-" + p.cur.Lexeme
			p.next()
			return &ast.TypeName{Name: name, Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
		}
		return &ast.TypeName{Name: "-", Span: ast.JoinSpan(start, spanPos(p.file, p.cur))}
	default:
		// Regular TypeName (IDENT, dotted paths, etc.)
		return p.parseTypeName()
	}
}
