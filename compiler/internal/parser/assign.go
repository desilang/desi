// compiler/internal/parser/assign.go
package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

/*** assignment or expression ***/

func (p *Parser) parseAssignOrExpr() (ast.Stmt, error) {
	// First ident is guaranteed by caller (parseStmt case).
	first, _ := p.expect(lexer.TokIdent)

	// Case 0: compound assignment for a single LHS identifier
	//   a += b
	//   a -= b
	//   a *= b
	//   a /= b
	//
	// We desugar into:
	//   AssignStmt{ Names: ["a"], Exprs: [ BinaryExpr(Ident("a"), "<op>", b) ] }
	if opTok, ok := p.acceptOneOf(lexer.TokPlusEq, lexer.TokMinusEq, lexer.TokStarEq, lexer.TokSlashEq); ok {
		rhs, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		nl, err := p.expect(lexer.TokNewline)
		if err != nil {
			return nil, err
		}

		leftIdent := &ast.IdentExpr{Name: first.Lex, Span: spanTok(first, first)}
		bin := &ast.BinaryExpr{
			Op:    compoundOpToBinary(opTok),
			Left:  leftIdent,
			Right: rhs,
			Span:  spanFrom(exprStart(leftIdent), exprEnd(rhs)),
		}

		return &ast.AssignStmt{
			Names: []string{first.Lex},
			Exprs: []ast.Expr{bin},
			Span:  spanTok(first, nl),
		}, nil
	}

	// Case 1: single assignment immediately: "a :="
	if p.accept(lexer.TokAssign) {
		exprs, err := p.parseExprListUntilNewline()
		if err != nil {
			return nil, err
		}
		nl, err := p.expect(lexer.TokNewline)
		if err != nil {
			return nil, err
		}
		return &ast.AssignStmt{
			Names: []string{first.Lex},
			Exprs: exprs,
			Span:  spanTok(first, nl),
		}, nil
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
		nl, err := p.expect(lexer.TokNewline)
		if err != nil {
			return nil, err
		}
		return &ast.AssignStmt{
			Names: names,
			Exprs: exprs,
			Span:  spanTok(first, nl),
		}, nil
	}

	// Case 3: not an assignment → this is an expression starting with that ident
	lhs := &ast.IdentExpr{Name: first.Lex, Span: spanTok(first, first)}
	expr, err := p.parseExprWithLHS(lhs)
	if err != nil {
		return nil, err
	}
	nl, err := p.expect(lexer.TokNewline)
	if err != nil {
		return nil, err
	}
	return &ast.ExprStmt{
		Expr: expr,
		Span: spanFrom(exprStart(expr), endPosFrom(nl)),
	}, nil
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

/*** small helpers ***/

// acceptOneOf tries to accept exactly one of the provided kinds.
// It returns the accepted kind and true on success; zero value and false otherwise.
func (p *Parser) acceptOneOf(kinds ...lexer.TokKind) (lexer.TokKind, bool) {
	for _, k := range kinds {
		if p.at(k) {
			p.next()
			return k, true
		}
	}
	return 0, false
}

// compoundOpToBinary maps a compound-assign token to its binary operator text.
func compoundOpToBinary(k lexer.TokKind) string {
	switch k {
	case lexer.TokPlusEq:
		return "+"
	case lexer.TokMinusEq:
		return "-"
	case lexer.TokStarEq:
		return "*"
	case lexer.TokSlashEq:
		return "/"
	default:
		return "" // should not happen
	}
}
