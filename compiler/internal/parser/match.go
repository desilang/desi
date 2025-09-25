package parser

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

// Grammar (Stage-1):
//
//	match <Ident> : NEWLINE
//	  INDENT
//	    <Variant> ( "(" <Ident>? ")" )? ":" NEWLINE
//	      INDENT
//	        <stmt>*
//	      DEDENT
//	    (more arms ...)
//	  DEDENT
//
// Additions in this version:
// - Wildcard arm: "_:" (no payload allowed).
// - Ignore-binder: Variant(_) — "_" is accepted as the (ignored) payload binder.
func (p *Parser) parseMatchStmtAt(matchTok lexer.Token) (*ast.MatchStmt, error) {
	// Stage-1: require scrutinee to be a bare identifier
	idTok, err := p.expect(lexer.TokIdent)
	if err != nil {
		return nil, err
	}
	scrut := &ast.IdentExpr{Name: idTok.Lex, Span: spanTok(idTok, idTok)}

	if _, err := p.expect(lexer.TokColon); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokNewline); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokIndent); err != nil {
		return nil, err
	}

	var arms []ast.MatchArm
	for !p.at(lexer.TokDedent) && !p.at(lexer.TokEOF) {
		// Variant name (including "_" for wildcard)
		vTok, err := p.expect(lexer.TokIdent)
		if err != nil {
			return nil, err
		}
		pat := ast.Pattern{Variant: vTok.Lex}
		endTok := vTok

		// Wildcard can't carry payload.
		if pat.Variant == "_" && p.at(lexer.TokLParen) {
			return nil, fmt.Errorf("wildcard '_' cannot have a payload")
		}

		// Optional "()" or "(name)" — binder can be "_" (ignored) or any identifier.
		if p.accept(lexer.TokLParen) {
			if p.at(lexer.TokIdent) {
				bTok, err := p.expect(lexer.TokIdent)
				if err != nil {
					return nil, err
				}
				pat.Bind = bTok.Lex // may be "_" (ignore-binder)
				endTok = bTok
			}
			rp, err := p.expect(lexer.TokRParen)
			if err != nil {
				return nil, err
			}
			endTok = rp
		}
		pat.Span = spanTok(vTok, endTok)

		if _, err := p.expect(lexer.TokColon); err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokIndent); err != nil {
			return nil, err
		}

		var body []ast.Stmt
		for !p.at(lexer.TokDedent) && !p.at(lexer.TokEOF) {
			st, err := p.parseStmt()
			if err != nil {
				return nil, err
			}
			body = append(body, st)
		}
		deTok, err := p.expect(lexer.TokDedent)
		if err != nil {
			return nil, err
		}

		arms = append(arms, ast.MatchArm{
			Pat:  pat,
			Body: body,
			Span: spanTok(vTok, deTok),
		})
	}

	deTok, err := p.expect(lexer.TokDedent)
	if err != nil {
		return nil, err
	}

	return &ast.MatchStmt{
		Scrut: scrut,
		Arms:  arms,
		Span:  spanTok(matchTok, deTok),
	}, nil
}
