package parser

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

func (p *Parser) ParseFile() (*ast.File, error) {
	f := &ast.File{}
	p.skipNewlines()

	// package (optional)
	if p.accept(lexer.TokPackage) {
		name, err := p.parseDottedIdent()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		f.Pkg = &ast.PackageDecl{Name: name}
		p.skipNewlines()
	}

	// imports (plain and from-imports)
	for p.at(lexer.TokImport) || p.at(lexer.TokFrom) {
		if p.at(lexer.TokImport) {
			importTok := p.tok
			p.next() // consume 'import'

			path, err := p.parseDottedIdent()
			if err != nil {
				return nil, err
			}

			// Optional: plain module aliasing — `import foo.bar as baz`
			var aliases []string
			if p.accept(lexer.TokAs) {
				aliasTok, err := p.expectIdent("module alias")
				if err != nil {
					return nil, err
				}
				aliases = append(aliases, aliasTok.Lex)
			}

			nlTok, err := p.expect(lexer.TokNewline)
			if err != nil {
				return nil, err
			}
			f.Imports = append(f.Imports, ast.ImportDecl{
				Path:    path,
				Aliases: aliases,
				Span:    spanTok(importTok, nlTok),
			})
		} else {
			// from-import
			fromTok := p.tok
			p.next() // consume 'from'
			fi, err := p.parseFromImportAt(fromTok)
			if err != nil {
				return nil, err
			}
			f.FromImports = append(f.FromImports, *fi)
		}
		p.skipNewlines()
	}

	// decls
	for !p.at(lexer.TokEOF) {
		// Optional 'pub' modifier before certain decls (M10 Phase A).
		seenPub := p.accept(lexer.TokPub)

		switch {
		case p.accept(lexer.TokDef):
			fn, err := p.parseFuncDecl()
			if err != nil {
				return nil, err
			}
			fn.Pub = seenPub
			f.Decls = append(f.Decls, fn)

		case p.at(lexer.TokType):
			// 'pub type' not supported yet; ignore 'pub' if present.
			typeTok := p.tok
			p.next() // consume 'type'
			td, err := p.parseTypeDeclAt(typeTok)
			if err != nil {
				return nil, err
			}
			// td has no Pub flag in AST (not part of M10 surface yet).
			f.Decls = append(f.Decls, td)

		case p.at(lexer.TokStruct):
			structTok := p.tok
			p.next() // consume 'struct'
			sd, err := p.parseStructDeclAt(structTok)
			if err != nil {
				return nil, err
			}
			sd.Pub = seenPub
			f.Decls = append(f.Decls, sd)

		case p.at(lexer.TokEnum):
			// 'pub enum' is planned later; ignore 'pub' if present.
			enumTok := p.tok
			p.next() // consume 'enum'
			ed, err := p.parseEnumDeclAt(enumTok)
			if err != nil {
				return nil, err
			}
			f.Decls = append(f.Decls, ed)

		default:
			// If we saw 'pub' but it's not followed by a supported decl, surface a clear error.
			if seenPub {
				t := p.tok
				return nil, fmt.Errorf("unexpected 'pub' before %q at %d:%d", t.Kind.String(), t.Line, t.Col)
			}
			// Surface lexer errors immediately at top-level
			if p.at(lexer.TokErr) {
				t := p.tok
				return nil, fmt.Errorf("%s at %d:%d", t.Lex, t.Line, t.Col)
			}
			// Skip to newline/EOF to avoid infinite loop on unknown tokens.
			for !p.at(lexer.TokNewline) && !p.at(lexer.TokEOF) {
				p.next()
			}
			p.skipNewlines()
		}
	}
	return f, nil
}

func (p *Parser) parseDottedIdent() (string, error) {
	var parts []string
	t, err := p.expectIdent("module/name")
	if err != nil {
		return "", err
	}
	parts = append(parts, t.Lex)
	for p.accept(lexer.TokDot) {
		t, err := p.expectIdent("module/name")
		if err != nil {
			return "", err
		}
		parts = append(parts, t.Lex)
	}
	return strings.Join(parts, "."), nil
}

// from <module> import <name> [as <alias>] (',' <name> [as <alias>])*
func (p *Parser) parseFromImportAt(fromTok lexer.Token) (*ast.FromImportDecl, error) {
	mod, err := p.parseDottedIdent()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokImport); err != nil {
		return nil, err
	}
	var items []ast.ImportItem
	// At least one item
	for {
		nameTok, err := p.expectIdent("imported symbol")
		if err != nil {
			return nil, err
		}
		it := ast.ImportItem{Name: nameTok.Lex, Span: spanTok(nameTok, nameTok)}
		if p.accept(lexer.TokAs) {
			aliasTok, err := p.expectIdent("alias")
			if err != nil {
				return nil, err
			}
			it.As = aliasTok.Lex
			it.Span = spanTok(nameTok, aliasTok)
		}
		items = append(items, it)
		if p.accept(lexer.TokComma) {
			continue
		}
		break
	}
	nl, err := p.expect(lexer.TokNewline)
	if err != nil {
		return nil, err
	}
	return &ast.FromImportDecl{
		Module: mod,
		Items:  items,
		Span:   spanTok(fromTok, nl),
	}, nil
}

func (p *Parser) parseTypeUntil(stoppers ...lexer.TokKind) (string, error) {
	stop := make(map[lexer.TokKind]bool)
	for _, k := range stoppers {
		stop[k] = true
	}
	var b strings.Builder
	depthParen, depthBrack := 0, 0
	for {
		if depthParen == 0 && depthBrack == 0 && stop[p.tok.Kind] {
			break
		}
		switch p.tok.Kind {
		case lexer.TokEOF, lexer.TokNewline, lexer.TokColon:
			return strings.TrimSpace(b.String()), nil
		case lexer.TokLParen:
			depthParen++
		case lexer.TokRParen:
			if depthParen > 0 {
				depthParen--
			}
		case lexer.TokLBrack:
			depthBrack++
		case lexer.TokRBrack:
			if depthBrack > 0 {
				depthBrack--
			}
		}
		if p.tok.Lex != "" {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(p.tok.Lex)
		} else {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(p.tok.Kind.String())
		}
		p.next()
	}
	return strings.TrimSpace(b.String()), nil
}

/*** NEW: type alias parser (M6) ***/

// parseTypeDeclAt expects we've just consumed 'type'.
//
//	type <Ident> = <type> NEWLINE
func (p *Parser) parseTypeDeclAt(typeTok lexer.Token) (*ast.TypeDecl, error) {
	nameTok, err := p.expectIdent("type name")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokEq); err != nil {
		return nil, err
	}
	under, err := p.parseTypeUntil(lexer.TokNewline)
	if err != nil {
		return nil, err
	}
	nl, err := p.expect(lexer.TokNewline)
	if err != nil {
		return nil, err
	}

	return &ast.TypeDecl{
		Name:       nameTok.Lex,
		Underlying: under,
		Span:       spanTok(typeTok, nl),
	}, nil
}

/*** ------------------ helpers (friendlier ident errors) ------------------ ***/

// expectIdent is like expect(TokIdent) but produces a nicer message if we
// encounter a keyword where an identifier is required.
func (p *Parser) expectIdent(context string) (lexer.Token, error) {
	if p.at(lexer.TokIdent) {
		t := p.tok
		p.next()
		return t, nil
	}
	t := p.tok
	if isKeywordToken(t.Kind) {
		return lexer.Token{}, fmt.Errorf("DPE0002: keyword %q cannot be used as an identifier for %s at %d:%d",
			t.Lex, context, t.Line, t.Col)
	}
	// Fallback to the normal expect error (keeps original formatting/code).
	return p.expect(lexer.TokIdent)
}

func isKeywordToken(k lexer.TokKind) bool {
	switch k {
	case lexer.TokPackage,
		lexer.TokImport,
		lexer.TokFrom,
		lexer.TokAs,
		lexer.TokDef,
		lexer.TokType,
		lexer.TokStruct,
		lexer.TokEnum,
		lexer.TokMut,
		lexer.TokLet,
		lexer.TokIf,
		lexer.TokElif,
		lexer.TokElse,
		lexer.TokWhile,
		lexer.TokReturn,
		lexer.TokMatch,
		lexer.TokPub:
		return true
	default:
		return false
	}
}
