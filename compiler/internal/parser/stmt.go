package parser

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

func (p *Parser) parseStmt() (ast.Stmt, error) {
	// Surface lexer errors immediately inside statements
	if p.at(lexer.TokErr) {
		t := p.tok
		return nil, fmt.Errorf("%s at %d:%d", t.Lex, t.Line, t.Col)
	}

	switch {
	case p.accept(lexer.TokLet):
		return p.parseLetStmt()

	case p.at(lexer.TokIdent):
		// Could be: parallel assignment "a, b := ..." OR an expression starting with an ident.
		return p.parseAssignOrExpr()

	case p.accept(lexer.TokReturn):
		if p.at(lexer.TokNewline) {
			p.next()
			return &ast.ReturnStmt{Expr: nil}, nil
		}
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		return &ast.ReturnStmt{Expr: expr}, nil

	case p.accept(lexer.TokIf):
		ifs, err := p.parseIfStmt()
		if err != nil {
			return nil, err
		}
		return ifs, nil

	case p.accept(lexer.TokWhile):
		ws, err := p.parseWhileStmt()
		if err != nil {
			return nil, err
		}
		return ws, nil

	case p.accept(lexer.TokDefer):
		// Stage-0: defer <call-expr> NEWLINE
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		return &ast.DeferStmt{Call: expr}, nil

	// Stray elif/else at statement start: make it a clear parser error instead of
	// falling through to expression parsing.
	case p.at(lexer.TokElif), p.at(lexer.TokElse):
		return nil, ErrUnexpectedToken("statement", p.tok)

	default:
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		return &ast.ExprStmt{Expr: expr}, nil
	}
}
