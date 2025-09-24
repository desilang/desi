package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

// parseEnumDeclAt expects we've just consumed 'enum'.
//
// Grammar (M8 P1):
//
//	enum <Ident> ":" NEWLINE INDENT
//	  <Ident> [ ":" <type> ] NEWLINE
//	  ...
//	DEDENT
//
// Payload type is optional; if omitted (or later normalized to "none"),
// we store empty string in the AST (Payload=="") to mean "no payload".
func (p *Parser) parseEnumDeclAt(enumTok lexer.Token) (*ast.EnumDecl, error) {
	nameTok, err := p.expect(lexer.TokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokColon); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokNewline); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokIndent); err != nil {
		return nil, err
	}

	var vars []ast.EnumVariant
	for !p.at(lexer.TokDedent) && !p.at(lexer.TokEOF) {
		p.skipNewlines()
		if p.at(lexer.TokDedent) || p.at(lexer.TokEOF) {
			break
		}

		vnameTok, err := p.expect(lexer.TokIdent)
		if err != nil {
			return nil, err
		}
		payload := ""
		endTok := vnameTok

		if p.accept(lexer.TokColon) {
			ty, err := p.parseTypeUntil(lexer.TokNewline)
			if err != nil {
				return nil, err
			}
			payload = ty // normalization happens in checker
		}
		nl, err := p.expect(lexer.TokNewline)
		if err != nil {
			return nil, err
		}
		endTok = nl

		vars = append(vars, ast.EnumVariant{
			Name:    vnameTok.Lex,
			Payload: payload,
			Span:    spanTok(vnameTok, endTok),
		})
	}

	ded, err := p.expect(lexer.TokDedent)
	if err != nil {
		return nil, err
	}
	return &ast.EnumDecl{
		Name:     nameTok.Lex,
		Variants: vars,
		Span:     spanTok(enumTok, ded),
	}, nil
}
