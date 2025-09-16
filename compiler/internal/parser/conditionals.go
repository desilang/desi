package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

func (p *Parser) parseIfStmt() (*ast.IfStmt, error) {
	// Back-compat path if called directly: approximate start at current token.
	return p.parseIfStmtAt(p.tok)
}

func (p *Parser) parseIfStmtAt(ifTok lexer.Token) (*ast.IfStmt, error) {
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
	lastEnd := blockEnd(thenBody)
	for p.at(lexer.TokElif) {
		elifTok := p.tok
		p.next()

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
		el := ast.ElseIf{
			Cond: ec,
			Body: eb,
			Span: spanFrom(posFrom(elifTok), blockEnd(eb)),
		}
		node.Elifs = append(node.Elifs, el)
		lastEnd = blockEnd(eb)
	}

	// optional else
	if p.at(lexer.TokElse) {
		elseTok := p.tok
		p.next()
		if _, err := p.expect(lexer.TokColon); err != nil {
			return nil, err
		}
		eb, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		node.Else = eb
		// else-branch end is the overall end
		lastEnd = blockEnd(eb)
		// we could record an internal span for 'else' if needed later using elseTok
		_ = elseTok
	}

	// Whole-if span: from 'if' to end of last arm
	node.Span = spanFrom(posFrom(ifTok), lastEnd)
	return node, nil
}

func (p *Parser) parseWhileStmt() (*ast.WhileStmt, error) {
	return p.parseWhileStmtAt(p.tok)
}

func (p *Parser) parseWhileStmtAt(whileTok lexer.Token) (*ast.WhileStmt, error) {
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
	return &ast.WhileStmt{
		Cond: cond,
		Body: body,
		Span: spanFrom(posFrom(whileTok), blockEnd(body)),
	}, nil
}
