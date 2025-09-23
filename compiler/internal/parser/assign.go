package parser

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

/*** assignment or expression ***/

func (p *Parser) parseAssignOrExpr() (ast.Stmt, error) {
	// First ident is guaranteed by caller (parseStmt case).
	first, _ := p.expect(lexer.TokIdent)

	// Parse a single LHS "term": ident with optional ".field" chain.
	lhs0, legacy0, err := p.parseLHSTermFrom(first)
	if err != nil {
		return nil, err
	}

	// Case 0: compound assignment ONLY for plain identifiers (Stage-1).
	//   a += b  (supported)
	//   a.b += c (NOT supported in Stage-1; treated as normal expr unless ':=' present)
	if _, isIdent := lhsAsIdent(lhs0); isIdent {
		if opTok, ok := p.acceptOneOf(lexer.TokPlusEq, lexer.TokMinusEq, lexer.TokStarEq, lexer.TokSlashEq); ok {
			rhs, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			nl, err := p.expect(lexer.TokNewline)
			if err != nil {
				return nil, err
			}

			leftIdent := lhs0.(*ast.IdentExpr)
			bin := &ast.BinaryExpr{
				Op:    compoundOpToBinary(opTok),
				Left:  leftIdent,
				Right: rhs,
				Span:  spanFrom(exprStart(leftIdent), exprEnd(rhs)),
			}

			return &ast.AssignStmt{
				LHS:   []ast.Expr{leftIdent},
				Names: []string{legacy0}, // legacy compat
				Exprs: []ast.Expr{bin},
				Span:  spanTok(first, nl),
			}, nil
		}
	}

	// Possibly a parallel assignment: a, b, u.id := ...
	lhsExprs := []ast.Expr{lhs0}
	legacyNames := []string{legacy0}
	sawComma := false

	if p.accept(lexer.TokComma) {
		sawComma = true
		for {
			term, legacy, err := p.parseLHSTerm()
			if err != nil {
				return nil, err
			}
			lhsExprs = append(lhsExprs, term)
			legacyNames = append(legacyNames, legacy)
			if p.accept(lexer.TokComma) {
				continue
			}
			break
		}
	}

	// If we see ':=', it's definitely an assignment to the (possibly multi) LHS.
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
			LHS:   lhsExprs,
			Names: legacyNames,
			Exprs: exprs,
			Span:  spanTok(first, nl),
		}, nil
	}

	// No ':=' — if we had commas, that's a syntax error; otherwise it's an expr stmt.
	if sawComma {
		return nil, ErrExpectedToken("assignment", lexer.TokAssign, p.tok)
	}

	// Expression starting with the already-parsed LHS head (ident/field chain).
	expr, err := p.parseExprWithLHS(lhs0)
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

// Parse an LHS term that *starts* with an identifier token we've already consumed.
func (p *Parser) parseLHSTermFrom(first lexer.Token) (ast.Expr, string, error) {
	var e ast.Expr = &ast.IdentExpr{Name: first.Lex, Span: spanTok(first, first)}
	legacy := first.Lex // becomes "" if a dot-chain is found
	for p.accept(lexer.TokDot) {
		id, err := p.expect(lexer.TokIdent)
		if err != nil {
			return nil, "", err
		}
		e = &ast.FieldExpr{
			X:    e,
			Name: id.Lex,
			Span: spanFrom(exprStart(e), endPosFrom(id)),
		}
		legacy = "" // not a plain ident anymore
	}
	return e, legacy, nil
}

// Parse an LHS term when the next token is an identifier.
func (p *Parser) parseLHSTerm() (ast.Expr, string, error) {
	id, err := p.expect(lexer.TokIdent)
	if err != nil {
		return nil, "", err
	}
	return p.parseLHSTermFrom(id)
}

// acceptOneOf tries to accept exactly one of the provided kinds.
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

// helper: check if LHS is a plain identifier
func lhsAsIdent(e ast.Expr) (*ast.IdentExpr, bool) {
	id, ok := e.(*ast.IdentExpr)
	return id, ok
}
