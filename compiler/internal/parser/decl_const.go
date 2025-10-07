package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

/*** NEW (M10): top-level const parsing ***/

// parseTopConstAt parses a single-name top-level constant:
//
//	let NAME (":" Type)? "=" <expr> NEWLINE
//
// When pub=true, the returned ConstDecl has Pub set.
func (p *Parser) parseTopConstAt(letTok lexer.Token, pub bool) (*ast.ConstDecl, error) {
	// Optional 'mut' (parser accepts; checker will forbid when pub)
	mutable := p.accept(lexer.TokMut)

	// Single name (no tuple/group syntax at top level)
	idTok, err := p.expectIdent("const name")
	if err != nil {
		return nil, err
	}

	annType := ""
	if p.accept(lexer.TokColon) {
		t, err := p.parseTypeUntil(lexer.TokEq)
		if err != nil {
			return nil, err
		}
		annType = t
	}

	if _, err := p.expect(lexer.TokEq); err != nil {
		return nil, err
	}

	// Reuse expression list, but require exactly one value.
	values, err := p.parseExprListUntilNewline()
	if err != nil {
		return nil, err
	}
	nl, err := p.expect(lexer.TokNewline)
	if err != nil {
		return nil, err
	}
	if len(values) != 1 {
		// Use a generic, but still-structured diag. Pin at the newline token.
		return nil, ErrTrailingOrExtraToken("top-level const", nl)
	}

	return &ast.ConstDecl{
		Name:    idTok.Lex,
		Type:    annType,
		Value:   values[0],
		Pub:     pub,
		Mutable: mutable,
		Span:    spanTok(letTok, nl),
	}, nil
}
