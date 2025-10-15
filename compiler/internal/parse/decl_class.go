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

	// Optional 'pub'
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
	bodyStart := spanPos(p.file, p.cur)
	if !p.expect(token.Indent, "indent") {
		return nil
	}

	var (
		fields  []*ast.FieldDecl
		methods []*ast.FuncDecl
		nested  []*ast.ClassDecl
		doc     *ast.StrLit
	)

	for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
		p.skipNLs()
		if p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
			break
		}

		// Docstring: single leading LONGSTR attaches and is dropped
		if doc == nil && p.cur.Tok == token.LONGSTR {
			doc = &ast.StrLit{Long: true, Span: spanPos(p.file, p.cur)}
			p.next()
			_ = p.accept(token.NL) // tolerate missing NL after long string
			continue
		}

		// Optional decorators
		memDecs := p.parseDecorators()
		p.skipNLs() // IMPORTANT: stay on the header token after decorator NL

		// Decide member kind (nested class, method, or field)
		if p.cur.Tok == token.KW_class || (p.cur.Tok == token.KW_pub && p.peek.Tok == token.KW_class) {
			if c := p.parseClassWithDecs(memDecs, true /*nested*/); c != nil {
				nested = append(nested, c)
			}
			continue
		}

		// Method?
		if p.cur.Tok == token.KW_def || p.cur.Tok == token.KW_async ||
			(p.cur.Tok == token.KW_pub && (p.peek.Tok == token.KW_def || p.peek.Tok == token.KW_async)) {
			if m := p.parseMethodWithDecs(memDecs); m != nil {
				methods = append(methods, m)
			}
			continue
		}

		// Field: ["pub"] Ident ":" TypeName NL
		fieldPub := p.accept(token.KW_pub)
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
			Pub:  fieldPub,
			Name: fname,
			Type: ty,
			Span: ast.JoinSpan(fname.Span, ty.Span),
		})
	}

	_ = p.expect(token.Dedent, "dedent")

	decl := &ast.ClassDecl{
		Pub:        explicitPub || !isNested, // top-level default public
		Name:       name,
		Bases:      bases,
		Fields:     fields,
		Methods:    methods,
		Nested:     nested,
		Decorators: decs,
		Doc:        doc,
		Span:       ast.JoinSpan(start, bodyStart),
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
		Params:     params,
		RetType:    ret,
		Body:       body,
		Decorators: decs,
		Span:       ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}

	// Attach leading docstring from the body if present.
	if fn.Body != nil && len(fn.Body.Stmts) > 0 {
		if ds, ok := fn.Body.Stmts[0].(*ast.DocStringStmt); ok && ds.Value != nil && ds.Value.Long {
			fn.Doc = ds.Value
			fn.Body.Stmts = fn.Body.Stmts[1:]
		}
	}

	return fn
}
