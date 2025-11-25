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

		// Check for variadic '*' prefix
		variadic := false
		if p.accept(token.STAR) {
			variadic = true
			if seenVariadic {
				p.errExpected(spanPos(p.file, p.cur), "only one variadic parameter allowed")
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

		if variadic && def != nil {
			p.errExpected(def.SpanOf(), "variadic parameter cannot have default value")
		}

		out = append(out, ast.Param{
			Name:     name,
			Type:     ty,
			Default:  def,
			Mode:     paramMode,
			Variadic: variadic,
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
	if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "type name")
		return &ast.TypeName{Name: "<?", Span: spanPos(p.file, p.cur)}
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

	// Parse type parameters: name[T1, T2, ...]
	var params []*ast.TypeName
	if p.accept(token.LBRACK) {
		for {
			param := p.parseTypeName()
			params = append(params, param)

			if !p.accept(token.COMMA) {
				break
			}
		}
		p.expect(token.RBRACK, "]")
	}

	return &ast.TypeName{
		Name:   b.String(),
		Params: params,
		Span:   ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}
}
