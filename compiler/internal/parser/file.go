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

	// imports
	for p.accept(lexer.TokImport) {
		path, err := p.parseDottedIdent()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		f.Imports = append(f.Imports, ast.ImportDecl{Path: path})
		p.skipNewlines()
	}

	// decls
	for !p.at(lexer.TokEOF) {
		switch {
		case p.accept(lexer.TokDef):
			fn, err := p.parseFuncDecl()
			if err != nil {
				return nil, err
			}
			f.Decls = append(f.Decls, fn)

		case p.at(lexer.TokType):
			typeTok := p.tok
			p.next() // consume 'type'
			td, err := p.parseTypeDeclAt(typeTok)
			if err != nil {
				return nil, err
			}
			f.Decls = append(f.Decls, td)

		case p.at(lexer.TokStruct):
			structTok := p.tok
			p.next() // consume 'struct'
			sd, err := p.parseStructDeclAt(structTok)
			if err != nil {
				return nil, err
			}
			f.Decls = append(f.Decls, sd)

		default:
			// Surface lexer errors immediately at top-level
			if p.at(lexer.TokErr) {
				t := p.tok
				return nil, fmt.Errorf("%s at %d:%d", t.Lex, t.Line, t.Col)
			}
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
	t, err := p.expect(lexer.TokIdent)
	if err != nil {
		return "", err
	}
	parts = append(parts, t.Lex)
	for p.accept(lexer.TokDot) {
		t, err := p.expect(lexer.TokIdent)
		if err != nil {
			return "", err
		}
		parts = append(parts, t.Lex)
	}
	return strings.Join(parts, "."), nil
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
// Grammar:
//
//	type <Ident> = <type> NEWLINE
func (p *Parser) parseTypeDeclAt(typeTok lexer.Token) (*ast.TypeDecl, error) {
	nameTok, err := p.expect(lexer.TokIdent)
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
