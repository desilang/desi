package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseStructWithDecs parses: [Decorators] ["pub"] "struct" Ident ":" NL Indent { docstring | field } Dedent
func (p *Parser) parseStructWithDecs(decs []*ast.Decorator) *ast.StructDecl {
	start := spanPos(p.file, p.cur)
	if len(decs) > 0 {
		start = decs[0].Span
	}
	// Optional 'pub' before 'struct'
	explicitPub := p.accept(token.KW_pub)

	if !p.expect(token.KW_struct, "struct") {
		return nil
	}
	if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "struct name")
		return nil
	}
	name := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
	p.next()

	if !p.expect(token.COLON, ":") {
		p.syncStmt()
		return nil
	}
	if !p.expect(token.NL, "newline") {
		p.syncStmt()
		return nil
	}
	if !p.expect(token.Indent, "indent") {
		return nil
	}

	var (
		fields []*ast.FieldDecl
		doc    *ast.StrLit
	)

	for p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
		p.skipNLs()
		if p.cur.Tok == token.Dedent || p.cur.Tok == token.EOF {
			break
		}
		// Leading docstring attaches to decl and is removed.
		if doc == nil && p.cur.Tok == token.LONGSTR {
			doc = &ast.StrLit{Long: true, Span: spanPos(p.file, p.cur)}
			p.next()
			_ = p.accept(token.NL) // tolerate optional NL after long string
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
		// Field span joins name->type (or name->name if type missing)
		end := fname.Span
		if ty != nil {
			end = ty.Span
		}
		fields = append(fields, &ast.FieldDecl{
			Pub:  fieldPub,
			Name: fname,
			Type: ty,
			Span: ast.JoinSpan(fname.Span, end),
		})
	}

	_ = p.expect(token.Dedent, "dedent")

	return &ast.StructDecl{
		Pub:        explicitPub, // top-level default is private unless 'pub'
		Name:       name,
		Fields:     fields,
		Decorators: decs,
		Doc:        doc,
		Span:       ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}
}
