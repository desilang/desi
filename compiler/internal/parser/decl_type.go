package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

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
