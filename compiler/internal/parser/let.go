package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

/*** let (single or parallel) ***/

func (p *Parser) parseLetStmt() (ast.Stmt, error) {
	mut := p.accept(lexer.TokMut)

	var binds []ast.LetBind
	groupType := ""

	// Parenthesized LHS allows an optional group type: (a: A, b: B): TupleType
	if p.accept(lexer.TokLParen) {
		// One or more binds
		for {
			b, err := p.parseLetBind(lexer.TokComma, lexer.TokRParen, lexer.TokEq)
			if err != nil {
				return nil, err
			}
			binds = append(binds, b)
			if p.accept(lexer.TokComma) {
				continue
			}
			if _, err := p.expect(lexer.TokRParen); err != nil {
				return nil, err
			}
			break
		}
		// Optional group type after ')'
		if p.accept(lexer.TokColon) {
			ty, err := p.parseTypeUntil(lexer.TokEq)
			if err != nil {
				return nil, err
			}
			groupType = ty
		}
	} else {
		// Unparenthesized: mixed annotation allowed per name, no group type.
		// At least one binder.
		b, err := p.parseLetBind(lexer.TokComma, lexer.TokEq, lexer.TokNewline)
		if err != nil {
			return nil, err
		}
		binds = append(binds, b)
		for p.accept(lexer.TokComma) {
			b, err := p.parseLetBind(lexer.TokComma, lexer.TokEq, lexer.TokNewline)
			if err != nil {
				return nil, err
			}
			binds = append(binds, b)
		}
	}

	if _, err := p.expect(lexer.TokEq); err != nil {
		return nil, err
	}

	values, err := p.parseExprListUntilNewline()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokNewline); err != nil {
		return nil, err
	}

	return &ast.LetStmt{
		Mutable:   mut,
		Binds:     binds,
		GroupType: groupType,
		Values:    values,
	}, nil
}

// parseLetBind parses: Ident ( ":" Type )?
// It stops when encountering any stopper token at top-level (comma, rparen, eq, newline).
func (p *Parser) parseLetBind(stoppers ...lexer.TokKind) (ast.LetBind, error) {
	id, err := p.expect(lexer.TokIdent)
	if err != nil {
		return ast.LetBind{}, err
	}
	// Optional per-name type
	if p.accept(lexer.TokColon) {
		ty, err := p.parseTypeUntil(stoppers...)
		if err != nil {
			return ast.LetBind{}, err
		}
		return ast.LetBind{Name: id.Lex, Type: ty}, nil
	}
	return ast.LetBind{Name: id.Lex}, nil
}
