package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

func (p *Parser) parseFuncDecl() (*ast.FuncDecl, error) {
	// We are called after 'def' has already been consumed by the caller.
	// Grammar (M1):
	//   def <name> "(" params? ")" "->" type ":" ( NEWLINE INDENT stmts DEDENT | <single-line-stmt> )
	//
	// The single-line form ends at the first NEWLINE after the ':'.

	nameTok, err := p.expect(lexer.TokIdent)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokLParen); err != nil {
		return nil, err
	}

	var params []ast.Param
	if !p.accept(lexer.TokRParen) {
		for {
			id, err := p.expect(lexer.TokIdent)
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokColon); err != nil {
				return nil, err
			}
			ty, err := p.parseTypeUntil(lexer.TokComma, lexer.TokRParen)
			if err != nil {
				return nil, err
			}
			params = append(params, ast.Param{Name: id.Lex, Type: ty})
			if p.accept(lexer.TokComma) {
				continue
			}
			if _, err := p.expect(lexer.TokRParen); err != nil {
				return nil, err
			}
			break
		}
	}

	if _, err := p.expect(lexer.TokArrow); err != nil {
		return nil, err
	}
	ret, err := p.parseTypeUntil(lexer.TokColon)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokColon); err != nil {
		return nil, err
	}

	// M1 decision point: after ':'
	// If the very next token is NEWLINE, we parse an indented block (legacy behavior).
	// Otherwise, we parse a single statement on the same line and wrap it as the function body.
	var body []ast.Stmt
	if p.at(lexer.TokNewline) {
		body, err = p.parseBlock()
		if err != nil {
			return nil, err
		}
	} else {
		// Single-line body: parse exactly one statement; parseStmt() already
		// consumes the trailing NEWLINE, so nothing special to do here.
		stmt, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		body = []ast.Stmt{stmt}
	}

	return &ast.FuncDecl{
		Name:   nameTok.Lex,
		Params: params,
		Ret:    ret,
		Body:   body,
		// Span optional for now (M14 will tighten spans across decls)
	}, nil
}

func (p *Parser) parseBlock() ([]ast.Stmt, error) {
	if _, err := p.expect(lexer.TokNewline); err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokIndent); err != nil {
		return nil, err
	}
	var body []ast.Stmt
	for !p.at(lexer.TokDedent) && !p.at(lexer.TokEOF) {
		p.skipNewlines()
		if p.at(lexer.TokDedent) || p.at(lexer.TokEOF) {
			break
		}
		s, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		body = append(body, s)
	}
	if _, err := p.expect(lexer.TokDedent); err != nil {
		return nil, err
	}
	return body, nil
}
