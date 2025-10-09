package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

// from <module> import <name> [as <alias>] (',' <name> [as <alias>])*
// Also supports:
//
//	from <module> import ( <name> [as <alias>] (',' <name> [as <alias>])* [','] )
func (p *Parser) parseFromImportAt(fromTok lexer.Token) (*ast.FromImportDecl, error) {
	mod, err := p.parseDottedIdent()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokImport); err != nil {
		return nil, err
	}

	var items []ast.ImportItem
	endTok := fromTok

	parenMode := p.accept(lexer.TokLParen)
	parseOneItem := func() (bool, error) {
		// In paren mode, allow newlines between items
		if parenMode {
			p.skipNewlines()
			// If we hit a right paren, caller should stop
			if p.at(lexer.TokRParen) {
				return true, nil
			}
		}

		// Expect a name
		nameTok, err := p.expectIdent("imported symbol")
		if err != nil {
			return false, err
		}
		it := ast.ImportItem{Name: nameTok.Lex, Span: spanTok(nameTok, nameTok)}
		endTok = nameTok

		// Optional "as alias"
		if p.accept(lexer.TokAs) {
			aliasTok, err := p.expectIdent("alias")
			if err != nil {
				return false, err
			}
			it.As = aliasTok.Lex
			it.Span = spanTok(nameTok, aliasTok)
			endTok = aliasTok
		}

		items = append(items, it)

		// Comma?
		if p.accept(lexer.TokComma) {
			if parenMode {
				// Trailing comma allowed: consume optional newlines, and if next is ')', stop
				p.skipNewlines()
				if p.at(lexer.TokRParen) {
					return true, nil
				}
			}
			// Continue parsing next item
			return false, nil
		}

		// No comma: done with items in flat mode; paren mode will handle ')' after returning
		return true, nil
	}

	// First item (required)
	if done, err := parseOneItem(); err != nil {
		return nil, err
	} else if !done {
		// keep consuming subsequent items
		for {
			done, err := parseOneItem()
			if err != nil {
				return nil, err
			}
			if done {
				break
			}
		}
	}

	if parenMode {
		// Optional newlines before ')'
		p.skipNewlines()
		rp, err := p.expect(lexer.TokRParen)
		if err != nil {
			return nil, err
		}
		endTok = rp
		// Optional newline after the parenthesized list
		if p.accept(lexer.TokNewline) {
			endTok = lexer.Token{Kind: lexer.TokNewline, Lex: "", Line: rp.Line, Col: rp.Col + 1}
		}
	} else {
		// Require newline at the end of a flat from-import
		nl, err := p.expect(lexer.TokNewline)
		if err != nil {
			return nil, err
		}
		endTok = nl
	}

	return &ast.FromImportDecl{
		Module: mod,
		Items:  items,
		Span:   spanTok(fromTok, endTok),
	}, nil
}
