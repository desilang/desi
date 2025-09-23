// compiler/internal/parser/assign.go
package parser

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/lexer"
)

/*** assignment or expression ***/

func (p *Parser) parseAssignOrExpr() (ast.Stmt, error) {
	// First ident is guaranteed by caller (parseStmt case).
	first, _ := p.expect(lexer.TokIdent)

	// Case 0: compound assignment for a single LHS identifier (no dotted)
	//   a += b, a -= b, a *= b, a /= b
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
		// Legacy-only path is fine for compound: checker supports Names; codegen uses Names.
		return &ast.AssignStmt{
			Names: []string{first.Lex},
			Exprs: []ast.Expr{bin},
			Span:  spanTok(first, nl),
		}, nil
	}

	// Build an expression head from the leading identifier, optionally
	// extending it with a dotted field chain:  a.b.c
	var headExpr ast.Expr = &ast.IdentExpr{Name: first.Lex, Span: spanTok(first, first)}
	lhsName := first.Lex
	consumedDot := false
	for p.accept(lexer.TokDot) {
		id, err := p.expect(lexer.TokIdent)
		if err != nil {
			return nil, err
		}
		consumedDot = true
		lhsName = lhsName + "." + id.Lex
		headExpr = &ast.FieldExpr{
			X:    headExpr,
			Name: id.Lex,
			Span: spanFrom(exprStart(headExpr), endPosFrom(id)),
		}
	}

	// Single (possibly dotted) assignment:  a := e   |   a.b := e   |   a.b.c := e
	if p.accept(lexer.TokAssign) {
		exprs, err := p.parseExprListUntilNewline()
		if err != nil {
			return nil, err
		}
		nl, err := p.expect(lexer.TokNewline)
		if err != nil {
			return nil, err
		}
		// Fill both the new structured LHS and legacy Names for maximum compatibility.
		return &ast.AssignStmt{
			LHS:   []ast.Expr{headExpr},
			Names: []string{lhsName},
			Exprs: exprs,
			Span:  spanTok(first, nl),
		}, nil
	}

	// Parallel assignment only allowed for bare identifiers (no dotted lhs)
	if !consumedDot && p.accept(lexer.TokComma) {
		var names []string
		var lhsExprs []ast.Expr
		names = append(names, first.Lex)
		lhsExprs = append(lhsExprs, &ast.IdentExpr{Name: first.Lex, Span: spanTok(first, first)})

		for {
			id, err := p.expect(lexer.TokIdent)
			if err != nil {
				return nil, err
			}
			// reject dotted in parallel LHS for now
			if p.at(lexer.TokDot) {
				return nil, ErrUnexpectedToken("parallel assignment LHS", p.tok)
			}
			names = append(names, id.Lex)
			lhsExprs = append(lhsExprs, &ast.IdentExpr{Name: id.Lex, Span: spanTok(id, id)})
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
			LHS:   lhsExprs,
			Names: names,
			Exprs: exprs,
			Span:  spanTok(first, nl),
		}, nil
	}

	// Not an assignment → this is an expression that starts with our headExpr
	expr, err := p.parseExprWithLHS(headExpr)
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
		return ""
	}
}

// keep imports happy if strings is otherwise unused in certain builds
var _ = strings.Builder{}
