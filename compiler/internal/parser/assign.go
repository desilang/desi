package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

/*** assignment or expression ***/

func (p *Parser) parseAssignOrExpr() (ast.Stmt, error) {
	// First ident is guaranteed by caller (parseStmt case).
	first, _ := p.expect(lexer.TokIdent)

	// Case 1: single assignment immediately: "a :="
	if p.accept(lexer.TokAssign) {
		exprs, err := p.parseExprListUntilNewline()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		return &ast.AssignStmt{Names: []string{first.Lex}, Exprs: exprs}, nil
	}

	// Case 2: parallel assignment starting "a, ..."
	if p.accept(lexer.TokComma) {
		var names []string
		names = append(names, first.Lex)

		for {
			id, err := p.expect(lexer.TokIdent)
			if err != nil {
				return nil, err
			}
			names = append(names, id.Lex)
			if p.accept(lexer.TokComma) {
				continue
			}
			break
		}
		if _, err := p.expect(lexer.TokAssign); err != nil {
			return nil, err
		}
		exprs, err := p.parseExprListUntilNewline()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokNewline); err != nil {
			return nil, err
		}
		return &ast.AssignStmt{Names: names, Exprs: exprs}, nil
	}

	// Case 3: not an assignment → this is an expression starting with that ident
	lhs := &ast.IdentExpr{Name: first.Lex}
	expr, err := p.parseExprWithLHS(lhs)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokNewline); err != nil {
		return nil, err
	}
	return &ast.ExprStmt{Expr: expr}, nil
}

func (p *Parser) parseExprListUntilNewline() ([]ast.Expr, error) {
	var xs []ast.Expr
	// Require at least one expr.
	e, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	xs = append(xs, e)
	for p.accept(lexer.TokComma) {
		// Trailing comma before NEWLINE/EOF is allowed.
		if p.at(lexer.TokNewline) || p.at(lexer.TokEOF) {
			break
		}
		// Detect ", ," (extra comma) early and surface DPE0004.
		if p.at(lexer.TokComma) {
			return nil, ErrTrailingOrExtraToken("assignment expression list", p.tok)
		}
		e2, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		xs = append(xs, e2)
	}
	return xs, nil
}
