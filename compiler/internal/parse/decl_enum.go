package parse

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/token"
)

// parseEnumWithDecs parses: [Decorators] ["pub"] "enum" Ident ":" NL Indent { docstring | variant } Dedent
// Variant: Ident ":" (TypeName | "none") NL
func (p *Parser) parseEnumWithDecs(decs []*ast.Decorator) *ast.EnumDecl {
	start := spanPos(p.file, p.cur)
	if len(decs) > 0 {
		start = decs[0].Span
	}
	// Optional 'pub' before 'enum'
	explicitPub := p.accept(token.KW_pub)

	if !p.expect(token.KW_enum, "enum") {
		return nil
	}
	if p.cur.Tok != token.IDENT {
		p.errExpected(spanPos(p.file, p.cur), "enum name")
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
		variants []*ast.EnumVariantDecl
		doc      *ast.StrLit
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

		// Variant: Ident ":" (TypeName | "none") NL
		if p.cur.Tok != token.IDENT {
			p.errExpected(spanPos(p.file, p.cur), "variant name")
			p.syncStmt()
			continue
		}
		vname := ast.Ident{Name: p.cur.Lexeme, Span: spanPos(p.file, p.cur)}
		p.next()
		if !p.expect(token.COLON, ":") {
			p.syncStmt()
			continue
		}
		var vty *ast.TypeName
		if p.cur.Tok == token.KW_none {
			// nil payload
			p.next()
		} else {
			vty = p.parseTypeName()
		}
		if !p.accept(token.NL) && p.cur.Tok != token.Dedent && p.cur.Tok != token.EOF {
			p.errExpected(spanPos(p.file, p.cur), "newline")
		}
		end := vname.Span
		if vty != nil {
			end = vty.Span
		}
		variants = append(variants, &ast.EnumVariantDecl{
			Name: vname,
			Type: vty,
			Span: ast.JoinSpan(vname.Span, end),
		})
	}

	_ = p.expect(token.Dedent, "dedent")

	return &ast.EnumDecl{
		Pub:        explicitPub, // top-level default is private unless 'pub'
		Name:       name,
		Variants:   variants,
		Decorators: decs,
		Doc:        doc,
		Span:       ast.JoinSpan(start, spanPos(p.file, p.cur)),
	}
}
