package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

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
