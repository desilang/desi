package parser

import (
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
		// Variant name
		vTok, err := p.expect(lexer.TokIdent)
		if err != nil {
			return nil, err
		}
		pat := ast.Pattern{Variant: vTok.Lex}
		endTok := vTok

		// Optional "()" or "(name)"
		if p.accept(lexer.TokLParen) {
			// payload binding is optional (for None() we allow empty parens)
			if p.at(lexer.TokIdent) {
				bTok, err := p.expect(lexer.TokIdent)
				if err != nil {
					return nil, err
				}
				pat.Bind = bTok.Lex
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
