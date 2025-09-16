package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

func (p *Parser) parseIfStmt() (*ast.IfStmt, error) {
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokColon); err != nil {
		return nil, err
	}
	thenBody, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	node := &ast.IfStmt{Cond: cond, Then: thenBody}

	// zero or more elif
	for p.accept(lexer.TokElif) {
		ec, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokColon); err != nil {
			return nil, err
		}
		eb, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		node.Elifs = append(node.Elifs, ast.ElseIf{Cond: ec, Body: eb})
	}

	// optional else
	if p.accept(lexer.TokElse) {
		if _, err := p.expect(lexer.TokColon); err != nil {
			return nil, err
		}
		eb, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		node.Else = eb
	}
	return node, nil
}

func (p *Parser) parseWhileStmt() (*ast.WhileStmt, error) {
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokColon); err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return &ast.WhileStmt{Cond: cond, Body: body}, nil
}
