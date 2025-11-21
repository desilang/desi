package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseTrait parses a 'trait Name: ...' declaration.
func (p *Parser) parseTrait(decs []*ast.Decorator) *ast.TraitDecl {
	start := spanPos(p.file, p.cur)
	if len(decs) > 0 {
		start = decs[0].Span
	}

	pub := p.accept(token.KW_pub)
	if !p.expect(token.KW_trait, "trait") {
		return nil
	}

	if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "trait name")
		return nil
	}
	name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
	p.next()

	if !p.expect(token.COLON, ":") {
		return nil
	}
	p.expect(token.NL, "newline")

	if !p.expect(token.Indent, "indent") {
		return nil
	}

	var methods []*ast.FuncDecl
	var doc *ast.StrLit

	for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
		p.skipNLs()
		if p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
			break
		}

		// Docstring?
		if p.cur.Tok == token.LONGSTR {
			if e := p.parseExpr(); e != nil {
				if lit, ok := e.(*ast.StrLit); ok {
					if doc == nil {
						doc = lit
					}
				}
			}
			continue
		}

		// Method signature
		mDecs := p.parseDecorators()
		// Traits usually have public methods by default or interface-like.
		// For now, we parse them like functions but expect no body (or empty body).
		// We'll reuse parseFuncWithDecs but ensure it doesn't require a body if it's a trait signature.
		// Actually, parseFuncWithDecs expects a body. Let's parse a signature manually or adjust parseFunc.
		// For M14, let's assume trait methods look like 'def foo(): ...' or 'def foo() -> T' followed by NL.

		// We'll use a simplified parseTraitMethod.
		if m := p.parseTraitMethod(mDecs); m != nil {
			methods = append(methods, m)
		} else {
			p.syncStmt()
		}
	}
	p.expect(token.Dedent, "dedent")

	return &ast.TraitDecl{
		Pub:        pub,
		Name:       name,
		Methods:    methods,
		Decorators: decs,
		Doc:        doc,
		Span:       ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}
}

func (p *Parser) parseTraitMethod(decs []*ast.Decorator) *ast.FuncDecl {
	start := spanPos(p.file, p.cur)
	if len(decs) > 0 {
		start = decs[0].Span
	}

	// 'def' is required
	if !p.expect(token.KW_def, "def") {
		return nil
	}

	if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "method name")
		return nil
	}
	name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
	p.next()

	if !p.expect(token.LPAREN, "(") {
		return nil
	}
	var params []ast.Param
	if p.cur.Tok != token.RPAREN {
		params = p.parseParams()
	}
	p.expect(token.RPAREN, ")")

	var ret *ast.TypeName
	if p.accept(token.ARROW) {
		ret = p.parseTypeName()
	}

	// Trait methods in M14 are signatures only. Expect NL.
	p.expect(token.NL, "newline")

	return &ast.FuncDecl{
		Name:       name,
		Params:     params,
		RetType:    ret,
		Decorators: decs,
		Span:       ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}
}

// parseImpl parses 'impl Trait for Type: ...'
func (p *Parser) parseImpl(decs []*ast.Decorator) *ast.ImplDecl {
	start := spanPos(p.file, p.cur)
	if !p.expect(token.KW_impl, "impl") {
		return nil
	}

	traitName := p.parseTypeName()

	if !p.expect(token.KW_for, "for") {
		return nil
	}

	targetType := p.parseTypeName()

	if !p.expect(token.COLON, ":") {
		return nil
	}
	p.expect(token.NL, "newline")
	if !p.expect(token.Indent, "indent") {
		return nil
	}

	var methods []*ast.FuncDecl
	for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
		p.skipNLs()
		if p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
			break
		}

		mDecs := p.parseDecorators()
		// Impl methods must have bodies.
		if f := p.parseFuncWithDecs(mDecs); f != nil {
			methods = append(methods, f)
		} else {
			p.syncStmt()
		}
	}
	p.expect(token.Dedent, "dedent")

	return &ast.ImplDecl{
		Trait:   traitName,
		ForType: targetType,
		Methods: methods,
		Span:    ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}
}
