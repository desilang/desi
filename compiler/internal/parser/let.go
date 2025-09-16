package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

/*** let (single or parallel) ***/

// Back-compat wrapper (in case anything else called parseLetStmt previously).
// When used, we won't have the real 'let' token; we approximate from current.
func (p *Parser) parseLetStmt() (ast.Stmt, error) {
	approxStart := p.tok
	return p.parseLetStmtAt(approxStart)
}

func (p *Parser) parseLetStmtAt(letTok lexer.Token) (ast.Stmt, error) {
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
	nl, err := p.expect(lexer.TokNewline)
	if err != nil {
		return nil, err
	}

	return &ast.LetStmt{
		Mutable:   mut,
		Binds:     binds,
		GroupType: groupType,
		Values:    values,
		Span:      spanTok(letTok, nl), // from 'let' to end-of-line
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
		// Span currently covers just the identifier (Stage-0). We can expand to include
		// the type once parseTypeUntil returns the last token position.
		return ast.LetBind{Name: id.Lex, Type: ty, Span: spanTok(id, id)}, nil
	}
	return ast.LetBind{Name: id.Lex, Span: spanTok(id, id)}, nil
}
